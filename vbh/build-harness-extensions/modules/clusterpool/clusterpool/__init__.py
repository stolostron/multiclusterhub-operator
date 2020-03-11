import os as _os
import socket
from time import gmtime, strftime


VERSION = '0.1'

RETURN_TRIGGER = 'trigger-return'
CLEANUP_TRIGGER = 'trigger-cleanup'
INSTANCE_NAME = 'cicd-pipeline'

def get_pickle(platform=''):
    return _os.path.normpath(_os.path.join(_os.path.dirname(
    _os.path.realpath(__file__)), '../state/{}_state.pickle'.format(platform)))

def is_travis_job():
    return _os.getenv('TRAVIS_JOB_ID', None) is not None and _os.getenv('TRAVIS_REPO_SLUG', None) is not None


def identifying_info():
    """Get the identifying information for the currently running process

    Returns:
        str -- unique string for the current process
    """
    if is_travis_job():
        travis_url = 'https://travis.ibm.com/{}/jobs/{}'.format(
            _os.getenv('TRAVIS_REPO_SLUG'), _os.getenv('TRAVIS_JOB_ID'))
        return '[{0}]({0})'.format(travis_url)

    return 'Locked by {} on {} ({})'.format(_os.getenv('GITHUB_USER'), socket.gethostname(), strftime("%Y-%m-%d %H:%M:%S", gmtime()))


def get_travis_github_token():
    return _os.getenv('GITHUB_TOKEN', None)


def creating_id():
    if is_travis_job():
        return _os.getenv('TRAVIS_JOB_ID')
    user = _os.getenv('GITHUB_USER').strip()
    if '%40' in user:
        return user.split('%40')[0]
    elif '@' in user:
        return user.split('@')[0]
    return user
