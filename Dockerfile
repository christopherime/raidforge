# Static site served by an unprivileged nginx (listens on :8080, runs as uid 101).
FROM nginxinc/nginx-unprivileged:1.27-alpine

# Build ref stamped into asset URLs for cache-busting (CI passes the short SHA).
ARG BUILD_REF=dev

# Custom config: serve on 8080 with a /healthz endpoint for k8s probes.
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Static assets.
COPY index.html /usr/share/nginx/html/index.html
COPY assets /usr/share/nginx/html/assets

# Version the asset query strings so each deploy is a fresh cache key at the
# browser AND Cloudflare's edge (which caches .css/.js by extension for hours).
USER root
RUN sed -i "s/__BUILD__/${BUILD_REF}/g" /usr/share/nginx/html/index.html
USER 101

EXPOSE 8080
