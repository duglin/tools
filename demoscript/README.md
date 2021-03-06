# Demoscript

A bash script that helps in demos.

Tired of having to type each command line during a demo? Tired of your typos?
Tired of having to remember which command comes next?  This little script
should help. Just `source` it in and then let the script do the typing for
you - even showing each character at a time if you want so it gives the visual
effect of typing. It'll:
- pause before and after each command, giving you time to explain what will
  happen, and what did happen
- show each command in bold so it stands out
- automatically `more` the output of commands so you don't have to scroll
  back up when there's a lot of output from a command
- works well with a presentation clicker - meaning you don't need to be near
  your laptop to run through the steps of the demo - pace around the stage
  all you want
- supports `wait`ing for the state of the system to be ready before
  moving on to the next step
- supports running in `SAVED` mode so you can run the demo and use a previous
  run's output - great for poor wifi situations
- supports running commands that require stdin - and yes you can
  specify that stdin if you wish

See `sample.sh` for an example of how it works.  Last example in there requires
docker to be installed.

Or a very short sample:
Original code:
```
#!/bin/bash

echo "This shows how to use 'tar'"
tar -cf test.tar *
tar -tf test.tar
rm test.tar
```

Demoscript enabled:
```
#!/bin/bash

source demoscript

comment "This shows how to use 'tar'"
doit tar -cf test.tar \*
doit tar -tf test.tar
doit rm test.tar
```

In the most basic case, just add `doit` before each command that you want
to talk about with the audience. All non-doit commands are just executed
as normal and hidden from the audience. Those are typically commands
use to do setup and clean-up.

We had to escape the `*` since `doit` is basically a wrapper and we don't
want the expanion to happen until `doit` runs the command. You can get the
same result by making it `doit 'tar -cf test.tar *'` if that's easier.

Using `comment` instead of `echo` is optional, but it makes it bold and
easier to see.

Each command's output is sent to a file called "out" so that you can
do some processing based on the previous comamnd's output - if needed.
This means you don't need to add `2>&1 | tee out` to each `doit` command.

## Environment variables

Set this environment variable to influence things...
- `DELAY` : Time to pause between each character displayed of command (0.02).
  To simulate typing speeds.
- `SAVE` : Save the output from each command in `script.tar` for later replay.
  "script" is replaced with the name of the bash file that `source`d
  `demoscript`
- `USESAVED` : Use the saved output in the tar file instead of running each
  command. This is good for off-line demos. But, keep in mind that since it
  doesn't actually the the command it doesn't change the state of anything.
  Which means it might not work properly if the a command requires a previous
  command to change something in the system - that change will not be there.
- `RETRYONFAIL` : Retry all `doit` commands that have unexcepted exit code.
- `DOCLEAR` : Clear the screen after each `doit` so that the next step will
  be shown at the top of the screen. Good for demos.

## Demoing

Each command that is run will be prefixed with the command prompt, `$`.

While running your script/demo you can tell it to stop pausing after each
character of the command by pressing `f` (fast) the next time it pauses.
If you press `r` (run) then it won't pause at all, so make sure you press
`f` before `r` if you want to do both.

## Scripting Commands

### `doit [ options ] COMMAND`
Runs a command.
- Pauses after the `$` (the prompt)
- Pauses after showing the command
- Shows the command in bold
- Sends the output of the command through `more` - screen size is reduced by 3
- Output of command (both stdout and stderr) are sent to a file called `out`.
  This file is erased when the bash script being executed exits.
- A list of the commands executed will be saved in a file called `cmds`.
- Script stops on unexcepted exit code

Options:
- `--ignorerc` : Ignores the exit code, even if it's non-zero
- `--shouldfail` : Script stops running on zero exit code
- `--noexec` : Don't actually execute the command, just show the command.
- `--usesaved` : Use the saved output (in the tar file) for just this one
- `--retryonfail` : Retry the command if there are unexpected results
- `--untilgrep=xx` : Run the command over and over until 'xx' is found in the
  output
- `--norepaint` : Don't over-write the output from a previous command when
  using `--untilgrep` - no cursor magic.
- `--slowoutput` : When USESAVED is turned on, this will slow down the output
  from the commands - otherwise it could be too fast for some demos
- `--pausetime=xx` : when using `--untilgrep` you can exit the loop by pressing
  any key. This waits `xx` seconds for a key before running the command again.
  I usually use a sub-second value.
- `--doclear` : Clear the screen afterwards (`--nodoclear` to disable it)

Example: `doit echo hello world`
<pre>
<b>$ echo hello world</b>
hello world
</pre>

### `background COMMAND`
Runs a command in the background.
- Shows the command in bold
- Executes it in the background
- Does not pause at all

Example: `background myserver`
<pre>
<b>$ myserver</b>
</pre>

### `ttyDoit [ OPTIONS ] COMMAND`
Runs a command that expects some input.
(Will probably be replaced soon with `doit --tty`)
- Reads the input for the command from fd 10
- Input typed for the command will be made bold
- Input lines that start with `@` will cause it to not pause before or after
sending that line

Options:
- `--ignorerc` : Ignores the exit code, even if it's non-zero
- `--shouldfail` : Script stops running on zero exit code

Example: 
```
ttyDoit myserver 10<<EOF
  input line 1
  input line 2
EOF
```
<pre>
<b>$ myserver</b>
Type some text: <b>input line 1</b>
And some more: <b>input line 2</b>
</pre>

### `comment SOME-TEXT`
Prints a comment to the screen, prefixed by `#`.
- Will not show the `#` if `--nohash` is specified
- Pauses after the `#` if `--pause` is specified
- Pauses after the text if `--pause` is specified
- Pauses only after the text if `--pauseafter` is specified
- `--nolf` will not print the blank line after the comment
- `--nocr` will not print the blank line or CR after the comment
- Shows the comment in bold

Options:
- `--nolf` : Do not print a blank line after the comment
- `--pause` : Pause after the `#` and the comment

Example: `comment About to do something cool`
<pre>
<b># About to do something cool</b>

</pre>

### `wait COMMAND`
Executes `command` over and over until it returns a non-zero exit code.
- Make sure you escape sh command - e.g. `wait curl ... \| grep ...`
- Nothing is shown to the screen

Example: `wait "curl http://myserver | grep 'HTTP/1.0 200`"

### `scroll FILE`
Prints a file to the screen through `more`. Similar to `doit cat FILE`
but does not treat it like a normal command, which means its output is not
saved when running in `SAVE` mode.
- Does not check exit code (yet)

Example: `scroll README`
<pre>
<b>$ more README</b>
This is a cool tool...
</pre>
