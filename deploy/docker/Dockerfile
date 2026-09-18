# All three base images are pinned tag@digest: a tag is mutable, so without the
# digest a rebuild of an identical commit can silently pull different bits.
# Renovate keeps both halves current (docker:pinDigests in renovate.json).
FROM node:24-alpine@sha256:50c8e8ca1d27439048670df5883f32d57cf81cff6233222c893fd0d9884cbd81 AS frontend
WORKDIR /app/frontend
# .npmrc comes along explicitly (package*.json doesn't glob it) so the image
# build honours ignore-scripts=true exactly like CI and local installs do —
# otherwise `npm ci` here would still run every dependency's install hooks.
COPY frontend/package*.json frontend/.npmrc ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS backend
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/importer ./cmd/importer && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate-avatars ./cmd/migrate-avatars

# static-debian13, not -debian12: the Debian release is part of the repository
# *name*, not the tag, so Renovate structurally cannot bump it — debian12 would
# sit here indefinitely on an ever-slower rebuild cadence. Both are pure static
# bases (CA certs + tzdata, no libc) and the binaries are CGO_ENABLED=0, so the
# switch is transparent to the app.
FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
COPY --from=frontend /app/frontend/dist /app/dist
COPY --from=backend /out/server /app/server
COPY --from=backend /out/importer /app/importer
COPY --from=backend /out/migrate-avatars /app/migrate-avatars
WORKDIR /app
# Stated explicitly rather than inherited from the :nonroot tag: with only the
# tag carrying it, a base-image change would silently ship a root container.
# 65532 is distroless' nonroot user, and matches runAsUser/runAsGroup in the
# Helm chart's securityContext.
USER 65532:65532
EXPOSE 8080
ENV STATIC_DIR=/app/dist
ENTRYPOINT ["/app/server"]
