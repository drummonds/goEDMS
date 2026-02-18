# Stage 1
FROM alpine:latest as build
RUN mkdir -p /opt/godocs/public/built && \
  mkdir /opt/godocs/config && \
  adduser -S godocs && addgroup -S godocs
WORKDIR /opt/godocs
COPY LICENSE README.md /opt/godocs/config/
COPY public/built/* /opt/godocs/public/built/
COPY dist/godocs_linux_amd64/godocs /opt/godocs/godocs
RUN chmod +x /opt/godocs/godocs && \
  chown -R godocs:godocs /opt/godocs/ && \
  apk update && apk add imagemagick tesseract-ocr

# Stage 2
FROM scratch
COPY --from=build / /
LABEL Author="drummonds"
LABEL name="godocs"
EXPOSE 8000
WORKDIR /opt/godocs
ENTRYPOINT [ "/opt/godocs/godocs" ]

#docker build -t deranjer/goedms:latest .
