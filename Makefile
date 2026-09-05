VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w -X github.com/trinhbentre/aiblame/internal/cli.Version=$(VERSION) \
           -X github.com/trinhbentre/aiblame/internal/cli.Commit=$(COMMIT) \
           -X github.com/trinhbentre/aiblame/internal/cli.Date=$(DATE)

.PHONY: build test lint fmt vet cover dist clean dogfood

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o aiblame ./cmd/aiblame

test:
	go test -race ./...

cover:
	go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint: vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)

# Cross-compile release binaries into dist/.
dist: clean
	@mkdir -p dist
	@for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
	  os=$${target%/*}; arch=$${target#*/}; ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
	  echo "building $$os/$$arch"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o dist/aiblame_$${os}_$${arch}$$ext ./cmd/aiblame || exit 1; \
	done
	@cd dist && for f in aiblame_*; do \
	  case $$f in *.exe) zip -q $${f%.exe}.zip $$f && rm $$f ;; *) tar czf $$f.tar.gz $$f && rm $$f ;; esac; \
	done && shasum -a 256 * > SHA256SUMS

clean:
	rm -rf dist aiblame aiblame.exe coverage.out

dogfood: build
	./aiblame stats . --no-color
