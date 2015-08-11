# kube-http-proxy

Easily run a reverse proxy for all your Kubernetes http and https traffic.

## Quick Start

First deploy a demo http server that we can proxy back to:

```console
kubectl run httpdemo --image=nginx --port=80
```

Now attach a service to it so that we can communicate with it (note that the
demo http server is serving on port 80, but we're exposing it on port 9090):

```console
kubectl expose rc httpdemo --port=9090 --target-port=80 --type=LoadBalancer
```

Here's an example YAML file for how we'd configure kube-http-proxy, save this
as `kube-http-proxy-rc.yml`:

```yaml
apiVersion: v1
kind: ReplicationController
metadata:
  name: kube-http-proxy
  labels:
    name: kube-http-proxy
spec:
  replicas: 1
  selector:
    name: kube-http-proxy
  template:
    metadata:
      labels:
        name: kube-http-proxy
    spec:
      containers:
      - name: kube-http-proxy
        image: ericflo/kube-http-proxy:latest
        ports:
        - name: http
          containerPort: 80
        args:
        - --domain=HTTPDEMO="example.com"
        - --domain=HTTPDEMO="www.example.com"
```

So we can load that up:

```console
kubectl create -f kube-http-proxy-rc.yml
```

Finally we can expose the http proxy:

```console
kubectl expose rc kube-http-proxy --port=80 --type=LoadBalancer
```

## Enabling SSL

Easy, just mount a certificate at `/etc/kube-http-proxy/certs/ssl.crt` and
your key at `/etc/kube-http-proxy/certs/ssl.key` and then make sure to expose
port 443 in the proxy service. Here's how that would look:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ssl-cert-secret
type: Opaque
data:
  ssl.crt: BASE64_ENCODED_SSL_CERTIFICATE
  ssl.key: BASE64_ENCODED_SSL_KEY
```

```yaml
apiVersion: v1
kind: ReplicationController
metadata:
  name: kube-http-proxy
  labels:
    name: kube-http-proxy
spec:
  replicas: 1
  selector:
    name: kube-http-proxy
  template:
    metadata:
      labels:
        name: kube-http-proxy
    spec:
      containers:
      - name: kube-http-proxy
        image: ericflo/kube-http-proxy:latest
        ports:
        - name: http
          containerPort: 80
        args:
        - --domain=HTTPDEMO="example.com"
        - --domain=HTTPDEMO="www.example.com"
        volumeMounts:
        - mountPath: /etc/kube-http-proxy/certs
          name: ssl-cert-vol
          readOnly: true
      volumes:
        - name: ssl-cert-vol
          secret:
            secretName: ssl-cert-secret
```

```yaml
kind: Service
apiVersion: v1
metadata:
  name: kube-http-proxy
spec:
  selector:
    name: kube-http-proxy
  ports:
    - name: http
      port: 80
      targetPort: 80
    - name: https
      port: 443
      targetPort: 443
  type: LoadBalancer
```