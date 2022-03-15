#Containerfile for project PubSub
FROM docker.io/library/golang:1-alpine AS build-env
WORKDIR /go/src/pubsub

#Let us cache modules retrieval as they do not change often.
#Better use of cache than go get -d -u
COPY go.mod .
COPY go.sum .
RUN go mod download

#Update certificates
RUN apk --update add ca-certificates

#Get source and build binary
COPY . .

#Path to main function
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /pubsub/bin

#Production image - scratch is the smallest possible but Alpine is a good second for bash-like access
FROM scratch
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /pubsub/bin /bin/pubsub

#Default root user container envars
ARG SUPERADMINUSER="ping"
ARG SUPERADMINPASSWORD="admin"
ARG PORT="8080"
ARG STORE="/store"
ENV PS_SUPERADMIN_USERNAME=${SUPERADMINUSER}
ENV PS_SUPERADMIN_PASSWORD=${SUPERADMINPASSWORD}
ENV PS_PORT=${PORT}
ENV PS_STORE=${STORE}

#Expose port for default API
EXPOSE 8080
#Expose port for SSE streams
EXPOSE 4039

VOLUME ["${STORE}"]

CMD ["/bin/pubsub"]