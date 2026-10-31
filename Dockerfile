# Consumed by GoReleaser dockers_v2: binaries live under $TARGETPLATFORM/.
FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM

WORKDIR /
USER nonroot:nonroot

COPY --chown=nonroot:nonroot $TARGETPLATFORM/xp-clean /xp-clean

ENTRYPOINT ["/xp-clean"]
