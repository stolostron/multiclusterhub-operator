
class ClusterException(Exception):
    pass

class ClusterDead(ClusterException):
    pass

class ClusterNotFound(ClusterException):
    pass

class ClusterNotInitialized(ClusterException):
    pass

class PipelineException(Exception):
    pass

class NoClustersAvailable(PipelineException):
    pass

