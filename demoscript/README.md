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
- Sends the output of the command through `more` - screen side is reduced by 3
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
command.

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
