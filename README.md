# Sherlock - Shared lock

Given the same script on multiple servers, Shelock ensures only one
will run at a given time. Particularly useful with cronjobs.

## Usage

    $ sherlock /usr/bin/python ~/myscript.py arg1 arg2

will run the script `/usr/bin/python ~/myscript.py arg1 arg2` and exit with the
same exit status as the script.

If other machine tries to execute the same line above it will wait
until the first one finishes.

It's possible to prevent other machines to run whatsoever with `-once`
parameter.

If N machines run the following line at the same time:

    $ sherlock -once python myscript.py arg1 arg2

Only one machine will succeed. Others will not run the script. Note
that this only prevents others from running while one of them is running.

See all available options with `-help`.

## License

```
Copyright (c) 2013 Igor Sobreira igor@igorsobreira.com

Permission is hereby granted, free of charge, to any person
obtaining a copy of this software and associated documentation
files (the "Software"), to deal in the Software without
restriction, including without limitation the rights to use,
copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the
Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE
OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
```

## Building the binaries

As you can see on bin/ directory there is a binary for linux/amd64.

Since I work on a Mac I need to cross compile sherlock to linux/amd64 to be able
to run on CentOS on our servers.

First you need to have [Go installed from source](http://golang.org/doc/install/source).
Then get the scripts for cross compilation:

    $ git clone git://github.com/davecheney/golang-crosscompile.git
    $ source golang-crosscompile/crosscompile.bash

Build Go for linux/amd64

    $ go-crosscompile-build linux/amd64

Now build sherlock for linux/amd64

    $ cd $GOPATH/src/github.com/realgeeks/sherlock
    $ go-linux-amd64 build -o bin/sherlock-linux-amd64

For more details on cross compilation in Go see
[this article](http://dave.cheney.net/2012/09/08/an-introduction-to-cross-compilation-with-go).
