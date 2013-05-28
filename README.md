# Sherlock - Shared lock

If one has the same program on multiple servers ensure only one is
executing at a given time, Sherlock can help.

## Usage

    $ distmutex python myscript.py arg1 arg2

will run the script 'python myscript.py arg1 arg2' and exit with the
same exit status as the script.

If other machine tries to execute the same line above it will wait
until the first one finishes.

It's possible to prevent other machines to run whatsoever with '-once'
parameter.

If `N` machines run the following line at the same time:

    $ distmutex -once python myscript.py arg1 arg2

Only one machine will succeed. Others will not run the script. Note
that this only prevents others from running while one of them is running.

This is useful to be used with cron.

## Some environment variables can be used

Sherlock uses memcache to store a mutex flag.

### `MEMCACHE_SERVERS`

A comma separared list of memcached servers to be used.

    $ MEMCACHE_SERVERS=server1:11211,server2:11211 distmutex python myscript.py

### `MEMCACHE_KEY`

The key to be used on memcache as the lock. Defaults to `mutex-default`

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
