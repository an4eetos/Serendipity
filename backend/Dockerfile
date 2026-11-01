# Builds the API into a static binary on a distroless image.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /server /server
# Railway sets PORT; without it the API listens on :8787.
EXPOSE 8787
ENTRYPOINT ["/server"]
