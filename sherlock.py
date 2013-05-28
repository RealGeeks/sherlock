#!/usr/bin/env python
#
# Copyright (c) 2013 Igor Sobreira igor@igorsobreira.com
#
# Permission is hereby granted, free of charge, to any person
# obtaining a copy of this software and associated documentation
# files (the "Software"), to deal in the Software without
# restriction, including without limitation the rights to use,
# copy, modify, merge, publish, distribute, sublicense, and/or
# sell copies of the Software, and to permit persons to whom the
# Software is furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be
# included in all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
# EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
# OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
# NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
# HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
# WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
# ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE
# OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
'''
Distributed mutex

Find documentation on the project page: http://github.com/realgeeks/sherlock

'''

import time
import sys
import os
import datetime
import logging
import functools
import traceback

import memcache

MEMCACHE = os.getenv('MEMCACHE_SERVERS', '127.0.0.1:11211')
KEY = os.getenv('MEMCACHE_KEY', 'mutex-default')


def logdebug(func):
    @functools.wraps(func)
    def wrapper(*args, **kw):
        logging.debug("Calling {0}".format(func.__name__))
        ret = func(*args, **kw)
        logging.debug("Returning {0}".format(func.__name__))
        return ret
    return wrapper

class AcquireDenied(Exception):
    pass

class MemcacheMutex(object):
    def __init__(self, key, retry=True):
        self.key = key
        self.retry = retry
        self.cli = memcache.Client(MEMCACHE.split(','))
        if not self.cli.get_stats():
            panic("No reachable memcache servers: {0}".format(MEMCACHE))
        logging.debug("Using memcache key '{0}'".format(key))
        logging.debug("Retry if lock is acquired: {0}".format(retry))

    @logdebug
    def acquire(self):
        while not self.cli.add(self.key, str(datetime.datetime.utcnow())):
            if not self.retry:
                logging.debug("Acquired by somebody else and retry disabled.")
                raise AcquireDenied
            logging.debug("Retrying")
            time.sleep(0.1)

    @logdebug
    def release(self):
        self.cli.delete(self.key)

def panic(err):
    logging.error(err)
    raise SystemExit(err)

def setup_logging():
    logging.basicConfig(
        format='%(levelname)s:%(asctime)s: %(message)s',
        stream=sys.stdout,
        level=logging.DEBUG,
    )

def run_process(args):
    ret = 0
    pid = os.fork()
    if pid == 0:
        logging.debug("Starting process")
        os.execvp(args[0], args)
    else:
        child, ret = os.waitpid(pid, 0)
        logging.debug("Process {0} exited with {1}".format(child, ret))
    return ret

def main():
    setup_logging()

    args = sys.argv[1:]
    if args[0] == '-once':
        once = True
        args.pop(0)
    else:
        once = False

    mutex = MemcacheMutex(KEY, retry=(not once))
    ret = 0
    try:
        mutex.acquire()
        ret = run_process(args)
    except AcquireDenied:
        pass
    except Exception:
        logging.error("Exception raised on fork/exec:\n{0}"
                      .format(traceback.format_exc()))
    finally:
        mutex.release()

    logging.shutdown()
    exit(ret)

if __name__ == '__main__':
    main()
