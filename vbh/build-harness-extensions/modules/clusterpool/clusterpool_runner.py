#!/usr/bin/env python3
# This script is used to run the cluster pool reservation system
import sys
import traceback

from clusterpool.main import main

if __name__ == '__main__':
    sys.exit(main())
