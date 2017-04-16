# Demoscript

A bash script that helps in demos.

See `sample.sh` for an example of how it works.  Last example in there requires
docker to be installed.

## Environment variables

Set this environment variable to influence things...
- `DELAY` : Time to pause between each character displayed of command (0.02).
To simulate typing speeds.
- `SAVE` : Save the output from each command in `script.tar` for later replay.
"script" is replaced with the name of the bash file that `source`d `demoscript`
- `USESAVED` : Use the saved output in the tar file instead of running each
command. This is good for off-line demos.

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
- Output of command (both stdout and stderr) are sent to a file called `out`
- Script stops on non-zero exit code

Options:
- `--ignorerc` : Ignores the exit code, even if it's non-zero
- `--shouldfail` : Script stops running on zero exit code
- `--noexec` : Don't actually execute the command, just show the command.
- `--usesaved` : Use the saved output (in the tar file) for just this one
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

### `command SOME-TEXT`
Prints a comment to the screen, prefixed by `#`.
- Pauses after the `#` only if `--pause` if specified
- Pauses after the text only if `--pause` if specified
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
