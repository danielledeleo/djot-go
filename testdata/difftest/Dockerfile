FROM golang:1.26-alpine
RUN apk add --no-cache git nodejs npm

# djot.js at the commit testdata/official was vendored from. The npm release
# (0.3.2) is older than the test files.
ARG DJOT_JS_COMMIT=9790d4db7953e7528927df26ff29bc7ec60bac06
WORKDIR /djot-js
RUN git init -q . && git remote add origin https://github.com/jgm/djot.js \
    && git fetch -q --depth 1 origin "$DJOT_JS_COMMIT" && git checkout -q FETCH_HEAD \
    && npm ci --no-audit --no-fund && npm run build

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
COPY testdata/difftest/render.mjs /app/render.mjs

ENTRYPOINT ["go", "test", "-v", "-tags", "difftest", "-run", "TestDifferential", "-count=1", "-timeout=300s", "."]
