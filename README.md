## StickyHeader is a plugin for [Traefik](https://github.com/traefik/traefik) that enables [sticky sessions](https://doc.traefik.io/traefik/routing/services/#sticky-sessions) based on headers.

### Plugin created by [nullxjx](https://github.com/nullxjx) just made some adjustments to the original code to make it work with treafik external plugin system.


## Introduction
Traefik uses cookies to implement [sticky sessions](https://doc.traefik.io/traefik/routing/services/#sticky-sessions), and by default, it doesn't support session persistence based on headers (such as IP Hash). However, some business scenarios require session persistence based on headers. For example, services like [vllm](https://github.com/vllm-project/vllm/) with [prefix-caching](https://docs.vllm.ai/en/v0.5.5/automatic_prefix_caching/apc.html) features may need to route requests from the same user to the same vllm pod, thereby improving the cache hit rate of the key-value (KV) cache in the GPU.

To address this, this plugin uses a local [LRU cache](https://github.com/hashicorp/golang-lru) to store the mapping between headers in requests and cookies set by Traefik. This allows Traefik’s cookie-based session persistence to be converted into header-based session persistence.
## Installation via helm chart

```yaml
experimental:
  plugins:
    stickyheader:
      moduleName: github.com/akierat/stickyheader
      version: v1.0.0
```

## Usage in IngressRoute

```yaml
apiVersion: traefik.io/v1alpha1
kind: IngressRoute
metadata:
  name: stickyheader-route
spec:
  entryPoints:
    - web
    - websecure
  routes:
    - kind: Rule
      match: Host(`example.pl`) && PathPrefix(`/example`)
      middlewares:
        - name: stickyheader-middleware
      services:
        - name: backend-service
          port: 80
          sticky:
            cookie:
              name: session_cookie
  tls:
    secretName: ssl-secret
```

## Usage in Middleware

```yaml
---
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: stickyheader-middleware
  namespace: dev-1
spec:
  plugin:
    stickyheader:
      cacheSize: 100
      headerName: X-Custom-Session
      cookieName: session_cookie
```



## Usage

1. Since [yaegi](https://github.com/traefik/yaegi) currently doesn't support Go modules, you need to download the repository's dependencies into your local `vendor` directory first:

    ```bash
    go mod tidy
    go mod download
    go mod vendor
    ```

2. Start the service:

    ```bash
    cd demo
    docker-compose up -d
    ```

    After executing the command, you should see in the Traefik logs that the plugin has loaded successfully:

    ![](./docs/start_up.png)

    Additionally, you can see the route in the Traefik dashboard where the plugin has been successfully applied:

    ![](./docs/dashboard_1.png)

    ![](./docs/dashboard_2.png)

3. Test the header-based session persistence:

    ```bash
    bash req.sh
    ```

    Modify the `X-USER-ID` value in the header, and observe the response to see if it switches to a different whoami pod.

## Limitations
Since the plugin uses a local [LRU cache](https://github.com/hashicorp/golang-lru), it becomes a stateful service. Therefore, it cannot be used with multiple Traefik pod replicas (in a Kubernetes cluster, for example). If you need to use multiple Traefik pod replicas, you'll need to replace the [LRU cache](https://github.com/hashicorp/golang-lru) with a shared cache like Redis.

### Partly workarround for multiple Traefik pod replicas inside Kubernetes cluster for svc
```yaml
  sessionAffinity: ClientIP
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 600
```
  


