#!/bin/bash

VERNO=$1
distro=$2

for p in $(find /var/lib/freight/apt/${distro}-testing -name "*_${VERNO}*.deb") ; do
	echo $(basename $p) | sed 's/\([^_]*\)_\([^_]*\)_.*/\1: \2/g'
done

