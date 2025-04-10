FROM gcr.io/distroless/static-debian12 as build
FROM scratch
COPY --from=build / /
COPY ./go-hl-val-mon /bin/go-hl-val-mon
ENTRYPOINT [ "go-hl-val-mon" ]
