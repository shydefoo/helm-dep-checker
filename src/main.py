from typing import Dict, List, Set


def chart_finder():
    pass


def construct_graph(input_charts: List) -> Dict:
    return {"root": ["a", "b"], "a": ["c", "d"], "b": ["d"], "c": ["e", "f"], "d": ["f"], "e": [], "f": []}


def get_root_chart() -> str:
    return "root"


def find_dependencies(graph: Dict, root_chart: str):
    def _f(chart):
        dep_list: List = graph[chart]
        if len(dep_list) == 0:
            return []
        child_dep_set: Set = set()
        common_dep: List = []
        for d in dep_list:
            child_deps = _f(d)
            for d in child_deps:
                if d not in child_dep_set:
                    child_dep_set.add(d)
                else:
                    common_dep.append(d)
        dep_list_set = set(dep_list)
        for d in common_dep:
            if d not in dep_list_set:
                dep_list_set.add(d)
        new_dep_list = list(dep_list_set)
        graph[chart] = new_dep_list
        # NOTE:
        # print statement here indicates changes need to be made to chart var Chart.yaml
        # and values.yaml. For example,
        # "a new deps ['c', 'd', 'f']" indicates
        # a Chart.yaml dependencies should have f added, a's values.yaml c.f.enabled: False and d.f.enabled: False
        # This means that the dependant chart needs to be stored
        print(f"{chart} new deps {new_dep_list}")
        return new_dep_list

    root_dep_list = _f(root_chart)
    return root_dep_list


if __name__ == "__main__":
    graph = construct_graph([])
    root = get_root_chart()
    root_dep_list = find_dependencies(graph, root)
