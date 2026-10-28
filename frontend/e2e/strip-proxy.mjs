// Minimal reverse proxy that strips a path prefix, like nginx
// `location /panel/ { proxy_pass http://127.0.0.1:8096/; }`.
// Usage: node e2e/strip-proxy.mjs <listen-port> <upstream-port> <prefix>
import http from 'node:http'

const [listenPort, upstreamPort, prefix] = process.argv.slice(2)

http
  .createServer((req, res) => {
    // nginx redirects "/panel" to "/panel/" for a `location /panel/` block.
    const bare = prefix.slice(0, -1)
    if (req.url === bare || req.url.startsWith(bare + '?')) {
      res.writeHead(301, { Location: prefix + req.url.slice(bare.length) }).end()
      return
    }
    if (!req.url.startsWith(prefix)) {
      res.writeHead(404).end('outside the proxied prefix')
      return
    }
    const upstream = http.request(
      {
        host: '127.0.0.1',
        port: Number(upstreamPort),
        method: req.method,
        path: '/' + req.url.slice(prefix.length),
        headers: { ...req.headers, host: req.headers.host },
      },
      (up) => {
        res.writeHead(up.statusCode ?? 502, up.headers)
        up.pipe(res)
      },
    )
    upstream.on('error', () => res.writeHead(502).end('upstream unavailable'))
    req.pipe(upstream)
  })
  .listen(Number(listenPort), '127.0.0.1')
