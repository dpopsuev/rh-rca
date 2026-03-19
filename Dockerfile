# RCA Engine — circuit MCP server.
# Reads domain data from a domain-serve endpoint via MCPRemoteFS.
#
# Build context must include sibling repos:
#   docker build -t origami-rca -f rh-rca/Dockerfile .   (from workspace root)
#
# Run: docker run -p 9200:9200 origami-rca --domain-endpoint http://domain:9300/mcp

FROM golang:1.24 AS builder
WORKDIR /src
COPY rh-rca/go.mod rh-rca/go.sum ./rh-rca/
COPY rh-dsr/go.mod rh-dsr/go.sum ./rh-dsr/
COPY origami/go.mod origami/go.sum ./origami/
RUN cd rh-rca && \
    go mod edit \
        -replace github.com/dpopsuev/origami=../origami \
        -replace github.com/dpopsuev/rh-dsr=../rh-dsr && \
    go mod download
COPY rh-rca/ ./rh-rca/
COPY rh-dsr/ ./rh-dsr/
COPY origami/ ./origami/
RUN cd rh-rca && CGO_ENABLED=0 go build -o /rca-serve ./cmd/serve

FROM gcr.io/distroless/base-debian12
COPY --from=builder /rca-serve /rca-serve
ENTRYPOINT ["/rca-serve"]
EXPOSE 9200
