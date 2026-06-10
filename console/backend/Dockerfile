# Build
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY static ./static
RUN CGO_ENABLED=0 go build -o /out/console .

# Run (distroless-style minimal)
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/console /console
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/console"]
