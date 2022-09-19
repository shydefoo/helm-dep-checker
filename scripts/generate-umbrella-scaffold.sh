#! /bin/bash

main() {
  target_dir=$1
  chart_names=("foo" "bar" "xyz" "abc" "def")
  if [[ -z $target_dir ]]; then
    mkdir -p $target_dir
  fi
  cd $target_dir
  for c in "${chart_names[@]}"; do
    helm create "$c"
  done
}
main $@
