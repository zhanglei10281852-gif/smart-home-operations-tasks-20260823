FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/smart-home ./cmd/server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/smart-home /smart-home
EXPOSE 8080
ENTRYPOINT ["/smart-home"]
