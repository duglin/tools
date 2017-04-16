#!/bin/bash

source $(dirname "$0")/demoscript

comment Just show some simple commands
doit ls -l
doit "ls -lR > output"
scroll output
rm output

comment Run a command and use the output
doit date
dd=$(sed "s/.........$//" < out)
doit echo The date w/o the year: $dd

comment Run a command that expects some input
doit docker run -d ubuntu sleep 60
cID=$(cat out)
ttyDoit docker exec -ti $cID bash 10<<EOF
	pwd
	ps -ef
	ls
	exit
EOF
doit docker rm -f $cID

rm out
rm cmds
