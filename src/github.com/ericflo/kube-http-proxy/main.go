package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/mailgun/oxy/forward"
)

func HostForService(service string) string {
	ip := os.Getenv(service + "_SERVICE_HOST")
	if ip == "" {
		log.WithField("service", service).Errorln("No service host found")
		return ""
	}
	port := os.Getenv(service + "_SERVICE_PORT")
	if port == "" {
		log.WithFields(log.Fields{
			"service": service,
			"ip":      ip,
		}).Errorln("No service port found")
		return ""
	}
	return ip + ":" + port
}

type hostFlags map[string]string

func (df *hostFlags) String() string {
	resp := make([]string, 0, len(*df))
	for domain, service := range *df {
		resp = append(resp, fmt.Sprintf("--domain=%s=%s", service, domain))
	}
	return strings.Join(resp, " ")
}

func (df *hostFlags) Set(value string) error {
	split := strings.Split(value, "=")
	if len(split) != 2 {
		log.Errorln("Invalid domain format: " + value)
		return nil
	}
	service := strings.ToUpper(strings.TrimSpace(split[0]))
	host := strings.ToLower(strings.TrimSpace(split[1]))
	if proxyHost := HostForService(service); proxyHost == "" {
		// If we got here, then we hit an error and already logged it, continue
		// by returning nil instead of an error
		return nil
	}
	(*df)[host] = service
	return nil
}

var hosts hostFlags = make(map[string]string, 0)
var sslCert string
var sslKey string

func init() {
	flag.Var(&hosts, "domain",
		"A domain/service mapping of the format SERVICENAME=\"example.com\"")
	flag.StringVar(&sslCert, "ssl-cert", "/etc/kube-http-proxy/certs/ssl.crt",
		"Location of your SSL certificate")
	flag.StringVar(&sslKey, "ssl-key", "/etc/kube-http-proxy/certs/ssl.key",
		"Location of your SSL certificate")
}

type Proxy struct {
	hosts map[string]string
	fwd   *forward.Forwarder
	log   *log.Entry
}

func NewProxy(hosts map[string]string) (*Proxy, error) {
	clog := log.WithFields(log.Fields{})
	fwd, err := forward.New(forward.Logger(clog))
	if err != nil {
		return nil, err
	}
	return &Proxy{
		hosts: hosts,
		fwd:   fwd,
		log:   clog,
	}, nil
}

func (p Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	split := strings.Split(strings.ToLower(req.Host), ":")
	if len(split) == 0 {
		http.Error(w, "Invalid hostname "+req.Host, 403)
		return
	}
	if service, ok := p.hosts[split[0]]; ok {
		host := HostForService(service)
		if host == "" {
			http.Error(w, "Could not look up service "+service, 403)
			return
		}
		req.URL.Scheme = "http"
		req.URL.Host = host
		p.fwd.ServeHTTP(w, req)
	} else {
		http.Error(w, "Unknown hostname "+req.Host, 403)
	}
}

func main() {
	flag.Parse()
	if len(hosts) == 0 {
		log.Fatalln("Must specify at least one domain/service mapping")
	}

	// Create the proxy
	proxy, err := NewProxy(hosts)
	if err != nil {
		log.WithField("err", err).Fatalln("Could not create proxy")
	}

	// Check to see if we can do SSL
	ssl := true
	if _, err := os.Stat(sslCert); os.IsNotExist(err) {
		ssl = false
	}
	if _, err := os.Stat(sslKey); os.IsNotExist(err) {
		ssl = false
	}

	// If we can, spin up a goroutine to listen for & proxy secure HTTPS conns
	if ssl {
		log.WithField("port", "443").Infoln("Starting HTTPS proxy")
		go func() {
			tlsServer := &http.Server{Addr: ":443", Handler: proxy}
			tlsServer.ListenAndServeTLS(sslCert, sslKey)
		}()
	}

	// On the main thread, listen for & proxy insecure HTTP conns
	log.WithField("port", "80").Infoln("Starting HTTP proxy")
	server := &http.Server{Addr: ":9000", Handler: proxy}
	server.ListenAndServe()
}
