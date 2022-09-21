from pathlib import Path
from typing import Dict, List

from chart import Chart


def construct_graph(input_charts: List[Chart]) -> Dict:
    return {"root": ["a", "b", "d"], "a": ["c", "d"], "b": ["d"], "c": ["e", "f"], "d": ["f"], "e": [], "f": []}


def get_root_chart() -> str:
    return "root"


def find_dependencies(graph: Dict, root_chart: str):
    dep_result_dict: Dict = {}

    def _f(chart):
        dep_list: List = graph[chart]
        if len(dep_list) == 0:
            return []
        # Stores mapping between child and list of parents
        child_dep_dict: Dict = {}
        common_dep: List = []
        for d in dep_list:
            child_deps = _f(d)
            for d_grand in child_deps:
                if d_grand not in child_dep_dict:
                    child_dep_dict[d_grand] = [d]
                else:
                    child_dep_dict[d_grand].append(d)
                    common_dep.append(d_grand)
        dep_list_set = set(dep_list)
        new_deps_added = False
        for d in common_dep:
            if d not in dep_list_set:
                dep_list_set.add(d)
                new_deps_added = True
        new_dep_list = list(dep_list_set)
        graph[chart] = new_dep_list
        # NOTE:
        # print statement here indicates changes need to be made to chart var Chart.yaml
        # and values.yaml. For example,
        # "root new deps ['b', 'd', 'a'], parent_look_up={'f': ['a'], 'd': ['a', 'b'], 'c': ['a']}" indicates
        # root Chart.yaml dependencies
        # should have deps [a,b,d], root's values.yaml a.d.enabled: False and b.d.enabled: False,
        # d.enabled: True (assuming dependency condition is d.enabled)
        if new_deps_added:
            dep_result_dict[chart] = {"full_deps": new_dep_list, "parent_look_up": child_dep_dict}
        return new_dep_list

    _ = _f(root_chart)
    return dep_result_dict


if __name__ == "__main__":
    import pprint

    graph = construct_graph([])
    root = get_root_chart()
    dep_result_dict = find_dependencies(graph, root)
    pprint.pprint(dep_result_dict)
