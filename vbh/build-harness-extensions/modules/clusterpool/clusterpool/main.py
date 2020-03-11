import argparse
import ast
import os
import pickle
import sys
import logging

from clusterpool import get_pickle, VERSION
from clusterpool.errors import ClusterDead, NoClustersAvailable
from clusterpool.clusterpool import Clusterpool

# create logger with 'clusterpool'
logger = logging.getLogger('clusterpool')
logger.setLevel(logging.INFO)
# console handler
ch = logging.StreamHandler()
ch.setLevel(logging.INFO)
# create formatter and add it to the handler
formatter = logging.Formatter('%(asctime)s - %(filename)s - %(levelname)s - %(message)s')
ch.setFormatter(formatter)
# add the handler to the logger
logger.addHandler(ch)

class ReadableDir(argparse.Action):
    def __call__(self, parser, namespace, values, option_string=None):
        prospective_dir = values
        if not os.path.isdir(prospective_dir):
            raise argparse.ArgumentTypeError(
                "ReadableDir:{0} is not a valid path".format(prospective_dir))
        if os.access(prospective_dir, os.R_OK):
            setattr(namespace, self.dest, prospective_dir)
        else:
            raise argparse.ArgumentTypeError(
                "ReadableDir:{0} is not a readable dir".format(prospective_dir))


def main(argv=sys.argv, stream=sys.stderr):
    logger.debug('main entry')
    parser, args = parse_args(argv)
    pickle_file = get_pickle(args.platform)
    if len(argv) > 1:
        if not os.path.isdir(os.path.dirname(pickle_file)):
            os.mkdir(os.path.dirname(pickle_file))
        args.func(args)
    else:
        parser.print_help()
        return 1
    return 0


def parse_args(argv):
    description = ''
    epilog = ''
    parser = argparse.ArgumentParser(description=description, epilog=epilog)
    parser.add_argument('-v', '--version', action='version',
                        version='%(prog)s {}'.format(VERSION))

    # add all subparers
    subparser = parser.add_subparsers()
    for func in [
        register_reserve_cluster_subcommand,
        register_configure_cluster_subcommand,
        register_get_info_subcommand,
        register_run_command_subcommand,
        register_return_cluster_subcommand,
        register_destroy_cluster_subcommand,
        register_upgrade_subcommand,
        register_check_queue_subcommand,
        register_clean_queue_subcommand
    ]:
        func(subparser)
    args = parser.parse_args(argv[1:])

    return parser, args


def register_reserve_cluster_subcommand(subparser):
    # repo_path
    # platform
    parser = subparser.add_parser('checkout',
                                  help='checkout a cluster from the queue')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.set_defaults(func=reserve_cluster)


def register_configure_cluster_subcommand(subparser):
    # repo_path
    # platform
    parser = subparser.add_parser('configure',
                                  help='configure the checked out cluster from the queue')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.add_argument('--kubectl-config-dir',
                        dest='kubectl_dir',
                        help='the directory for kubectl configurations to be stored relative to current directory. Defaults to \'~/.kubectl\'.',
                        default=None)
    parser.add_argument('--helm-config-dir',
                        dest='helm_dir',
                        help='the directory for helm configurations to be stored relative to current directory. Defaults to \'~/.helm\'.',
                        default=None)
    parser.add_argument('--name',
                        help='the name of a cluster to configure: will NOT check it out',
                        default=None)
    parser.set_defaults(func=configure_cluster)


def register_get_info_subcommand(subparser):
    # repo_path
    # platform
    parser = subparser.add_parser('get-reserved-info',
                                  help='get terraform output from the checked out cluster')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.add_argument('--output',
                        dest='output',
                        help='the terraform output variable name to pull for the cluster',
                        default=None)
    parser.add_argument('--length',
                        dest='output_length',
                        help='the number of lines expected in the output; defaults to 1.',
                        default=1)
    parser.set_defaults(func=get_info)


def register_run_command_subcommand(subparser):
    # repo_path
    # platform
    parser = subparser.add_parser('run-command',
                                  help='run commands on the checked out cluster from stdin')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.set_defaults(func=run_command)


def register_return_cluster_subcommand(subparser):
    # repo_path
    # platform
    parser = subparser.add_parser('return',
                                  help='return the your most recent checked out cluster')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.set_defaults(func=return_cluster)


def register_destroy_cluster_subcommand(subparser):
    # repo_path
    # platform
    # cluster
    # force
    # debug
    parser = subparser.add_parser('destroy',
                                  help='destroy a cluster')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.add_argument('--cluster',
                        help='the name of the cluster to destroy',
                        required=True)
    parser.add_argument('--force',
                        help='boolean to specify force destruction, even if cluster is checked out',
                        dest='force',
                        default=False)
    parser.add_argument('--debug',
                        help='boolean to specify debug logging level',
                        dest='debug',
                        default=False)
    parser.set_defaults(func=destroy_cluster)


def register_upgrade_subcommand(subparser):
    # repo_path
    # platform
    # desired_count
    parser = subparser.add_parser('upgrade',
                                  help='refresh all available clusters to the latest build')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.add_argument('--count',
                        help='the number of clusters present for this platform in the cluster pool',
                        type=int,
                        dest='desired_count',
                        required=True)
    parser.set_defaults(func=upgrade)


def register_check_queue_subcommand(subparser):
    # repo_path
    # platform
    # desired_count
    # should_clean
    parser = subparser.add_parser('check-queue',
                                  help='check to see if any clusterpool clusters need to be added or removed')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.add_argument('--count',
                        help='the number of clusters present for this platform in the clusterpool',
                        type=int,
                        dest='desired_count',
                        required=True)
    parser.add_argument('--commit-message',
                        help='message to parse. Clusters will be cleaned if message follows "return <clusterpool-label> <cluster-id>"',
                        dest='message',
                        default=None)
    parser.add_argument('--should-clean',
                        help='boolean to specify to just report the current status vs. act on mismatches',
                        dest='should_clean',
                        default=True)
    parser.set_defaults(func=check_queue)


def register_clean_queue_subcommand(subparser):
    # repo_path
    # platform
    # desired_count
    parser = subparser.add_parser('clean-queue',
                                  help='ensure all clusters in the cluster pool are healthy')
    parser.add_argument('--repo',
                        help='the fully qualified path to the cloned clusterpool repo',
                        action=ReadableDir,
                        dest='repo_path',
                        required=True)
    parser.add_argument('--platform',
                        help='the platform you want to deploy on (corresponding to a directory in the repo)',
                        required=True)
    parser.add_argument('--count',
                        help='the number of clusters present for this platform in the cluster pool',
                        type=int,
                        dest='desired_count',
                        required=True)
    parser.set_defaults(func=clean_queue)


def reserve_cluster(args):
    pickle_file = get_pickle(args.platform)
    if os.path.isfile(pickle_file):
        with open(pickle_file, 'rb') as f:
            clusterpool = pickle.load(f)
    else:
        clusterpool = Clusterpool(args.repo_path, args.platform)
    while True:
        try:
            reservation = clusterpool.checkout_cluster()
        except NoClustersAvailable as e:
            print(e)
            print('Contact CICD to request an increase in cluster count or try again later.')
            return 1
        print('reserved cluster: {}'.format(reservation.name))

        if not 'openshift4-aws' in clusterpool._reserved_cluster.platform:
            print('Configuring the cluster')
            try:
                reservation.configure()
            except ClusterDead:
                print('Cluster not responding; Trying to reserve another')
                clusterpool._reserved_cluster = None
                continue
        break
    print('Cluster: {}/{}'.format(clusterpool.platform, reservation.name))
    if 'openshift4-aws' in clusterpool.platform:
        print('Web console at:\n\thttps://{}'.format(reservation.icp_web_console))
        print('ssh to install node using the command:\n\tssh ec2-user@{} -i {}'.format(reservation.master_node, reservation.private_key_file))
    else:
        print('Web console at:\n\thttps://{}:8443'.format(reservation.master_node))
        print('ssh using the command:\n\tssh ubuntu@{} -i {}'.format(reservation.master_node, reservation.private_key_file))
    
    with open(pickle_file, 'wb') as f:
        pickle.dump(clusterpool, f)


def configure_cluster(args):
    pickle_file = get_pickle(args.platform)
    if args.name is not None:
        # We are trying to configure a cluster that is not checked out
        clusterpool = Clusterpool(args.repo_path, args.platform)
        cluster = clusterpool.get_cluster(args.name)
        cluster.configure(helm_config=args.helm_dir,
                          kubectl_config=args.kubectl_dir, override_claim=True)
        print('Cluster configured: {}/{}'.format(clusterpool.platform, cluster.name))
        return 0
    reservation = None
    if os.path.isfile(pickle_file):
        with open(pickle_file, 'rb') as f:
            clusterpool = pickle.load(f)
        reservation = clusterpool.reserved_cluster
    if reservation is None:
        print('You need to checkout a cluster before configuring it.')
        return 1
    print('configuring cluster: {}'.format(reservation.name))
    reservation.configure(helm_config=args.helm_dir,
                          kubectl_config=args.kubectl_dir)
    print('Cluster configured: {}/{}'.format(clusterpool.platform, reservation.name))
    print('Web console at: https://{}:8443'.format(reservation.master_node))
    print('ssh using the command:\n\tssh ubuntu@{} -i {}'.format(reservation.master_node, reservation.private_key_file))
    with open(pickle_file, 'wb') as f:
        pickle.dump(clusterpool, f)


def return_cluster(args):
    pickle_file = get_pickle(args.platform)
    if os.path.isfile(pickle_file):
        with open(pickle_file, 'rb') as f:
            clusterpool = pickle.load(f)
    else:
        clusterpool = Clusterpool(args.repo_path, args.platform)
    reservation = clusterpool.return_cluster()
    if reservation is None:
        print('No cluster reserved; return failed')
        return 1
    print('returned cluster: {}'.format(reservation.name))
    os.remove(pickle_file)


def destroy_cluster(args):
    clusterpool = Clusterpool(args.repo_path, args.platform)
    success = clusterpool.destroy_cluster(args.cluster, args.force, args.debug)


def upgrade(args):
    clusterpool = Clusterpool(args.repo_path, args.platform, args.desired_count)
    success = clusterpool.upgrade()


def check_queue(args):
    clusterpool = Clusterpool(args.repo_path, args.platform, args.desired_count)
    clusterpool.check(args.message, args.should_clean)


def clean_queue(args):
    clusterpool = Clusterpool(args.repo_path, args.platform, args.desired_count)
    clusterpool.cleanup()


def get_info(args):
    pickle_file = get_pickle(args.platform)
    reservation = None
    if os.path.isfile(pickle_file):
        with open(pickle_file, 'rb') as f:
            clusterpool = pickle.load(f)
        reservation = clusterpool.reserved_cluster
    if reservation is None:
        print('You need to checkout a cluster before getting output from it.')
        return 1
    if args.output is None:
        print('You need to specify an output to read')
        return 1
    print(reservation._get_terraform_output(args.output, length_of_output=args.output_length))


def run_command(args):
    pickle_file = get_pickle(args.platform)
    reservation = None
    if os.path.isfile(pickle_file):
        with open(pickle_file, 'rb') as f:
            clusterpool = pickle.load(f)
        reservation = clusterpool.reserved_cluster
    if reservation is None:
        print('You need to checkout a cluster before running commands on it.')
        return 1
    reservation.execute_shell_command(sys.stdin, timeout=30)
