#Containerfile for project PubSub
FROM golang:alpine AS build-env
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
RUN CGO_ENABLED=0 GOOS=linux GOARCH= amd64 go build -o /pubsub/bin

#Production image - scratch is the smallest possible but Alpine is a good second for bash-like access
FROM scratch
COPY --from=build-env /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build-env /pubsub/bin /bin/pubsub

#Default root user container envars
ARG SUPERADMINUSER="ping"
ARG SUPERADMINPASSWORD="admin"
ARG PORT="8080"
ENV SUPERADMINUSER=${SUPERADMINUSER}
ENV SUPERADMINPASSWORD=${SUPERADMINPASSWORD}
ENV PORT=${PORT}

EXPOSE 8080

CMD ["/bin/pubsub"]