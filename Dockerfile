FROM golang:1

ENV PROJECT=annotations-publisher
ENV BUILDINFO_PACKAGE="github.com/Financial-Times/service-status-go/buildinfo."

COPY . /${PROJECT}/
WORKDIR /${PROJECT}

ARG GITHUB_USERNAME
ARG GITHUB_TOKEN

RUN VERSION="version=$(git describe --tag --always 2> /dev/null)" \
  && DATETIME="dateTime=$(date -u +%Y%m%d%H%M%S)" \
  && REPOSITORY="repository=$(git config --get remote.origin.url)" \
  && REVISION="revision=$(git rev-parse HEAD)" \
  && BUILDER="builder=$(go version)" \
  && GOPRIVATE="github.com/Financial-Times" \
  && git config --global url."https://${GITHUB_USERNAME}:${GITHUB_TOKEN}@github.com".insteadOf "https://github.com" \
  && LDFLAGS="-s -w -X '"${BUILDINFO_PACKAGE}$VERSION"' -X '"${BUILDINFO_PACKAGE}$DATETIME"' -X '"${BUILDINFO_PACKAGE}$REPOSITORY"' -X '"${BUILDINFO_PACKAGE}$REVISION"' -X '"${BUILDINFO_PACKAGE}$BUILDER"'" \
  && mkdir -p /artifacts/schemas/ \
  && cp -r /${PROJECT}/schemas /artifacts/schemas \
  && mkdir -p /artifacts/config/ \
  && cp -r /${PROJECT}/config /artifacts/config \
  && CGO_ENABLED=0 go build -mod=readonly -a -o /artifacts/${PROJECT} -ldflags="${LDFLAGS}"


# Multi-stage build - copy only the certs and the binary into the image
FROM scratch

ENV PROJECT=annotations-publisher

WORKDIR /
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=0 /artifacts/* /
COPY --from=0 /${PROJECT}/api/api.yml /

CMD [ "/annotations-publisher" ]
