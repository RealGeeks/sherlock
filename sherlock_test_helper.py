'''
Helper for sherlock_test.go

This script will be called using sherlock during the tests, some command line
flags can be provided to change it's behavior and verify on test, like write
something to stdout or exit with specific status

How to use command line arguments
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

    $ python sherlock_test_helper.py stdout:"write this stdout"

this will set argument "stdout" with value "write this stdout"

Available command line arguments
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

    stdout:"a string to write to stdout"

    stderr:"a string to write to stderr"

    exitcode:10
        exit program with status code 10

    touchfile:/tmp/hi.txt
        write string "sherlock says hi" to /tmp/hi.txt

'''

from __future__ import print_function

import sys


ARGUMENTS = {
    'stdout': unicode,
    'stderr': unicode,
    'exitcode': int,
    'touchfile': unicode,
}

def main():
    args = parse_args()
    if 'stdout' in args:
        print(args['stdout'])
    if 'stderr' in args:
        print(args['stderr'], file=sys.stderr)
    if 'touchfile' in args:
        with open(args['touchfile'], 'w') as fobj:
            fobj.write(u"sherlock says hi")
    if 'exitcode' in args:
        sys.exit(args['exitcode'])

def parse_args():
    args = {}
    for arg in sys.argv[1:]:
        try:
            name, value = arg.split(':', 1)
        except ValueError:
            sys.exit("Invalid argument format: {0}".format(arg))
        try:
            arg_type = ARGUMENTS[name]
        except KeyError:
            sys.exit("Argument not available: {0}".format(arg))
        try:
            value = arg_type(value)
        except ValueError:
            sys.exit("Invalid argument type, expect {0} for {1}".format(arg_type, arg))
        args[name] = value
    return args


if __name__ == '__main__':
    main()
