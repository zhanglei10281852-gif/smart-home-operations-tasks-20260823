FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /out/smart-home ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/smart-home /smart-home
EXPOSE 8080
ENTRYPOINT ["/smart-home"]
