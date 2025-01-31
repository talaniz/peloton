import pytest

from tests.integration.job import (
    Job,
    with_instance_count,
)

# Mark test module so that we can run tests by tags
pytestmark = [
    pytest.mark.default,
    pytest.mark.job,
    pytest.mark.random_order(disabled=True),
]


# Create a job with 5 instances and small resource requirements
# such that all can run on the same host. Verify that they are
# packed onto the same host
def test_placement_strategy_pack():
    job = Job(
        job_file="test_task.yaml",
        options=[with_instance_count(5)])
    """
    TODO Uncomment next line after peloton-client changes
    #job.job_config.placementStrategy = "PLACEMENT_STRATEGY_PACK_HOST"
    """
    job.create()
    job.wait_for_state()

    # check all of them ran on same host
    the_host = ""
    task_infos = job.list_tasks().value
    for instance_id, task_info in task_infos.items():
        if the_host:
            assert task_info.runtime.host == the_host
        the_host = task_info.runtime.host


# Create a job with 3 instances and small resource requirements
# such that all can run on the same host. Verify that they are
# spread over different hosts
"""
TODO Re-enable test after peloton-client changes
def test_placement_strategy_spread():
    job = Job(
        job_file="test_task.yaml",
        options=[with_instance_count(3)])
    job.job_config.placementStrategy = "PLACEMENT_STRATEGY_SPREAD_JOB"
    job.create()
    job.wait_for_state()

    # check all of them ran on different hosts
    hosts = set()
    task_infos = job.list_tasks().value
    for instance_id, task_info in task_infos.items():
        assert task_info.runtime.host not in hosts
        hosts.add(task_info.runtime.host)
"""
