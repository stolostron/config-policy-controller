#! /bin/bash

set -e

cd "$(dirname "$0")"

passes=0

#makefile and imports both pass, no error expected
echo "Case 1: Makefile and imports do not have upstream references"
if (./../../detect_upstream.sh -m "cat mock_makefile_pass.txt" -i "cat mock_deps_output_pass.txt"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#makefile fails, imports pass, error expected
echo "Case 2: Makefile has upstream references"
if ! (./../../detect_upstream.sh -m "cat mock_makefile_fail.txt" -i "cat mock_deps_output_pass.txt"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#makefile passes, imports fail, error expected
echo "Case 3: Imports have upstream references"
if ! (./../../detect_upstream.sh -m "cat mock_makefile_pass.txt" -i "cat mock_deps_output_fail.txt"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

#makefile fails, imports fail, error expected
echo "Case 4: Makefile and imports have upstream references"
if ! (./../../detect_upstream.sh -m "cat mock_makefile_fail.txt" -i "cat mock_deps_output_fail.txt"); then
    echo "PASS"
    passes=$((passes + 1))
else
    echo "FAIL"
fi

echo ------------------------------------------------------
echo "Upstream script tests completed, ${passes}/4 passed"
if [[ $passes < 4 ]]; then
  exit 1
fi