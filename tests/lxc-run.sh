#!/bin/sh
# Runs the entrypoint the way an unprivileged LXC on Proxmox does: as root in a
# user namespace whose uid 0 is uid 100000 on the host. Needs a privileged
# container, and starts as the host's root to set the mounts up as the host
# sees them.
#
# Everything in Config and Data belongs to the container, except the paths in
# HOST_ROOT_OWNS, which stay 0:0 with mode 700, like the lost+found mke2fs
# leaves at the root of a Proxmox mount point. Afterwards OWNS lines report who
# owns what, and the tree goes back to OWNER so the test can read and delete it.
set -u

base=100000
set -- /etc/opencloud /var/lib/opencloud

chown -R "${base}:${base}" "$@"
for p in ${HOST_ROOT_OWNS:-}; do
    mkdir -p "${p}"
    chown 0:0 "${p}"
    chmod 700 "${p}"
done

# A placeholder holds the namespace while its map is written. The entrypoint
# joins it afterwards: a process that runs before the map exists loses its
# capabilities on exec.
unshare -U sleep 600 &
holder=$!
until [ "$(readlink "/proc/${holder}/ns/user")" != "$(readlink /proc/self/ns/user)" ]; do sleep 0.1; done
echo "0 ${base} 65536" >"/proc/${holder}/uid_map"
echo "0 ${base} 65536" >"/proc/${holder}/gid_map"
nsenter -U -t "${holder}" -S 0 -G 0 /usr/local/bin/entrypoint.sh
rc=$?
kill "${holder}"

find "$@" -maxdepth 1 -exec stat -c 'OWNS %u:%g %n' {} +
chown -R "${OWNER}" "$@"
exit "${rc}"
