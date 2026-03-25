FROM golang:1.26-alpine
RUN apk add --no-cache nodejs npm

WORKDIR /djot-js
RUN npm init -y >/dev/null 2>&1 && npm install @djot/djot

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
COPY testdata/difftest/render.mjs /app/render.mjs

ENTRYPOINT ["go", "test", "-v", "-tags", "difftest", "-run", "TestDifferential", "-count=1", "-timeout=300s", "."]
