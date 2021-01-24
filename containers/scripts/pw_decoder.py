#!/usr/bin/python2

# From https://github.com/redhataccess/redhat-support-tool/blob/master/src/redhat_support_tool/helpers/confighelper.py

from itertools import izip, cycle
import base64
import sys

def __xor(salt, string):
    '''
    A simple utility function to obfuscate the password when a user
    elects to save it in the config file.  We're not necessarily
    going for strong encryption here (eg. a keystore) because that
    would require a user supplied PW to unlock the keystore.
    We're merely trying to provide a convenience against an
    accidental display of the file (eg. cat ~/.rhhelp).
    '''
    str_ary = []
    for x, y in izip(string, cycle(salt)):
        str_ary.append(chr(ord(x) ^ ord(y)))
    return ''.join(str_ary)

def pw_decode(password, key):
    '''
    This convenience function will de-obfuscate a password or other value
    previously obfuscated with pw_encode()
    Returns the de-obfuscated string
    '''
    if password and key:
        password = base64.urlsafe_b64decode(password)
        return __xor(key, password)
    else:
        return None

def pw_encode(password, key):
    '''
    This convenience function will obfuscated a password or other value
    Returns the obfuscated string
    '''
    if password and key:
        passwd = __xor(key, password)
        return base64.urlsafe_b64encode(passwd)
    else:
        return None

if sys.argv[1] == "decode":
    print(pw_decode(sys.argv[3], sys.argv[2]))
if sys.argv[1] == "encode":
    print(pw_encode(sys.argv[3], sys.argv[2]))
