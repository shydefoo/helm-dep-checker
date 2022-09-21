from pathlib import Path
from typing import Dict, List

from yaml import SafeLoader, load


class Chart:
    """Chart."""

    def __init__(self, repo: str, path: str):
        """__init__.

        :param repo:
        :type repo: str
        :param path:
        :type path: str
        """
        self.repo: str = repo
        self.path: str = path
        self.chart_yaml: Dict = self.get_chart_yaml(path)
        self.values_yaml: Dict = self.get_values_yaml(path)
        self.chart_hash = self.get_chart_hash()

    @staticmethod
    def get_file(path: str, sub_path: str) -> Dict:
        """get_file.

        :param path:
        :type path: str
        :param sub_path:
        :type sub_path: str
        :rtype: Dict
        """
        chart_path: Path = Path(path) / sub_path
        if not chart_path.exists():
            return {}  # NOTE: empty dict indicates file does not exist, assume to be external dep
        with chart_path.open() as fs:
            chart_dict = load(fs, SafeLoader)
        return chart_dict

    @classmethod
    def get_chart_yaml(cls, path: str) -> Dict:
        """get_chart_yaml.

        :param path:
        :type path: str
        :rtype: Dict
        """
        return cls.get_file(path, "Chart.yaml")

    @classmethod
    def get_values_yaml(cls, path: str):
        """get_values_yaml.

        :param path:
        :type path: str
        """
        return cls.get_file(path, "values.yaml")

    def get_chart_hash(self) -> str:
        """get_chart_hash returns unique value that will be used to construct dependency graph

        :rtype: str
        """
        c = self.chart_yaml
        return "{repo}/{name}-{version}".format(repo=self.repo, name=c["name"], version=c["version"])


def get_dep_chart_list(c: Chart) -> List[Chart]:
    dep_charts: List[Chart] = []
    dep_list: List = c.chart_yaml.get("dependencies", [])
    if not dep_list:
        return dep_charts
    for d in dep_list:
        c = Chart(d["repository"], d["name"])
    return dep_charts


if __name__ == "__main__":
    c = Chart("caraml", "test-chart/abc")
    print(c.chart_yaml)
    print(c.values_yaml)
