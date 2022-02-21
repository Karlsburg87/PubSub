#!/usr/bin/env bash

#Find if programs are installed using command built-in function
#See: http://manpages.ubuntu.com/manpages/trusty/man1/bash.1.html (search: 'Run  command  with  args')
if [[ -x "$(command -v podman)" ]]; then
  podman build -t pubsub . && podman run -it -rm subsub
  #cleanup
  podman image rm pubsub
elif [[ -x "$(command -v docker)" ]]; then
  #One line build and execute 
  #https://stackoverflow.com/questions/45141402/build-and-run-dockerfile-with-one-command
  docker build -t pubsub . && docker run -it -rm subsub
  #cleanup
  docker image rm pubsub
else
  echo "You need to have either Docker Desktop or Podman to run"
fi