#!/usr/bin/env bash

#Process mangement: https://www.digitalocean.com/community/tutorials/how-to-use-bash-s-job-control-to-manage-foreground-and-background-processes

#Find if programs are installed using command built-in function
#See: http://manpages.ubuntu.com/manpages/trusty/man1/bash.1.html (search: 'Run  command  with  args')
if [[ -x "$(command -v podman)" ]]; then
  podman build -t pubsub .
  podman run -dt --rm --name pubsuber -p 8080:8080 -p 4039:4039 pubsub
  #cleanup
  podman image prune
elif [[ -x "$(command -v docker)" ]]; then
  #One line build and execute 
  #https://stackoverflow.com/questions/45141402/build-and-run-dockerfile-with-one-command
  docker build -t pubsub . 
  docker run -dt --rm --name pubsuber -p 8080:8080 -p 4039:4039 pubsub
  #cleanup
  docker image prune
else
  echo "You need to have either Docker Desktop or Podman to run"
fi

