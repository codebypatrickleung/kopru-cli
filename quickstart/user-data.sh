#!/bin/bash
# ==============================================================================
# Kopru CLI Instance Initialization Script
# ==============================================================================

set -euo pipefail

exec > >(tee /var/log/kopru-init.log)
exec 2>&1

# Configuration
MOUNTPOINT="/kopru"
EXPECTED_DISKS=4
VG_NAME="kopru_vg"
LV_NAME="kopru_lv"
DEVICE="/dev/${VG_NAME}/${LV_NAME}"
KOPRU_DIR="${MOUNTPOINT}/kopru-cli"
KOPRU_BRANCH="main"
KOPRU_REPO="https://github.com/codebypatrickleung/kopru-cli.git"

MAX_WAIT=120
INTERVAL=10

print_header() {
    echo "=========================================="
    echo "$1"
    echo "=========================================="
}

print_step() {
    echo ""
    echo "Step $1: $2"
}

print_header "Kopru CLI Instance Initialization - Started at: $(date)"

# Step 1: Configure LVM striped storage
print_step 1 "Configuring LVM striped storage on paravirtualised data disks"

dnf install -y lvm2 >/dev/null

echo "Waiting for ${EXPECTED_DISKS} paravirtualised data disks..."
shopt -s nullglob

ELAPSED=0
DATA_DISKS=()

while [[ "$ELAPSED" -lt "$MAX_WAIT" ]]; do
    DATA_DISKS=()
    for disk in /dev/sd[b-z]; do
        [[ -b "$disk" ]] && DATA_DISKS+=("$disk")
    done

    if [[ "${#DATA_DISKS[@]}" -ge "$EXPECTED_DISKS" ]]; then
        echo "Found ${#DATA_DISKS[@]} data disks"
        break
    fi

    echo "Found ${#DATA_DISKS[@]}/${EXPECTED_DISKS} disks, waiting... (${ELAPSED}s)"
    sleep "$INTERVAL"
    ELAPSED=$((ELAPSED + INTERVAL))
done

shopt -u nullglob

if [[ "${#DATA_DISKS[@]}" -lt "$EXPECTED_DISKS" ]]; then
    echo "Error: Expected ${EXPECTED_DISKS} disks but found ${#DATA_DISKS[@]}"
    lsblk -d -n -p -o NAME,SIZE,TYPE
    exit 1
fi

DATA_DISKS=("${DATA_DISKS[@]:0:$EXPECTED_DISKS}")
echo "Using disks: ${DATA_DISKS[*]}"

pvcreate "${DATA_DISKS[@]}"
vgcreate "${VG_NAME}" "${DATA_DISKS[@]}"
lvcreate -i "${EXPECTED_DISKS}" -I 64k -l 100%FREE -n "${LV_NAME}" "${VG_NAME}"
mkfs.xfs -f "${DEVICE}"
mkdir -p "${MOUNTPOINT}"
mount -o defaults,noatime "${DEVICE}" "${MOUNTPOINT}"

if ! grep -q "${DEVICE}" /etc/fstab; then
    echo "${DEVICE} ${MOUNTPOINT} xfs defaults,noatime,nofail 0 2" >> /etc/fstab
fi

echo "LVM storage mounted at ${MOUNTPOINT}"
df -h "${MOUNTPOINT}"

# Step 2: Install git
print_step 2 "Installing git"

if ! command -v git &>/dev/null; then
    dnf install -y git >/dev/null
    echo "Git installed"
fi

git --version

# Step 3: Clone kopru-cli repository
print_step 3 "Cloning kopru-cli repository"

[[ -d "$KOPRU_DIR" ]] && rm -rf "$KOPRU_DIR"
git clone -b "$KOPRU_BRANCH" "$KOPRU_REPO" "$KOPRU_DIR"
echo "Repository cloned successfully"

cd "$KOPRU_DIR"
echo "Working directory: $(pwd)"

# Step 4: Run setup-environment.sh
print_step 4 "Running setup-environment.sh"

if [[ -f "./scripts/setup-environment.sh" ]]; then
    chmod +x ./scripts/setup-environment.sh
    bash ./scripts/setup-environment.sh
else
    echo "Error: setup-environment.sh not found"
    exit 1
fi

# Step 5: Build kopru-cli
print_step 5 "Building kopru-cli from source"

export HOME="${HOME:-/root}"
export GOPATH="${GOPATH:-$HOME/go}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"

mkdir -p "$GOPATH" "$GOMODCACHE"

if [[ -f "go.mod" ]]; then
    go build -o kopru ./cmd/kopru
    chmod +x kopru
    cp kopru /usr/local/bin/kopru
    echo "Build completed"
else
    echo "Error: go.mod not found"
    exit 1
fi

# Step 6: Verify installation
print_step 6 "Verifying kopru-cli installation"

if [[ -x "./kopru" ]]; then
    ./kopru --version
    echo "Kopru CLI verified successfully"
else
    echo "Error: kopru binary not found or not executable"
    exit 1
fi

# Summary
print_header "Initialization Complete - Finished at: $(date)"
echo ""
echo "Summary:"
echo "  Data disk:         ${MOUNTPOINT}"
echo "  Repository:        $KOPRU_DIR"
echo "  Binary:            /usr/local/bin/kopru"
echo ""
echo "Run 'kopru --help' to get started"
