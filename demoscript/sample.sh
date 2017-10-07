#!/bin/bash

source $(dirname "$0")/demoscript

comment --nolf Use the spacebar to step through things...
comment --nolf Use 'f' to speed up the typing
comment Use 'r' to stop pausing

comment --pause Just show some simple commands
doit ls -l
doit "ls -lR > output"
scroll output
rm output

comment Run a command and use the output in a subsequent command
doit date
dd=$(sed "s/.........$//" < out)
doit echo The date w/o the year: $dd

comment Prep for next step...
doit docker pull ubuntu

comment Run a command that expects some input
doit docker run -d ubuntu sleep 60
cID=$(cat out)

comment Now we\'ll pause between each tty line of input to the command
ttyDoit docker exec -ti $cID bash 10<<EOF
	pwd
	ps -ef
	ls
	exit
EOF
doit docker rm -f $cID

rm out
rm cmds
