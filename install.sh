#!/bin/bash

set -e

# Tailstream Agent Installer
# Usage: curl https://install.tailstream.io | bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="tailstream-io/tailstream-agent"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/tailstream"
SERVICE_FILE="/etc/systemd/system/tailstream-agent.service"
BINARY_NAME="tailstream-agent"
USER_NAME="tailstream"

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root, escalate if needed
check_root() {
    if [[ $EUID -ne 0 ]]; then
        if [[ -t 0 ]]; then
            # Interactive terminal - can use sudo directly
            print_status "Installer requires root privileges, requesting sudo access..."
            exec sudo "$0" "$@"
        else
            # Piped input - need to save script and re-run
            print_status "Installer requires root privileges. Please run with sudo:"
            print_status "curl https://install.tailstream.sh | sudo bash"
            exit 1
        fi
    fi
}

# Detect architecture
detect_arch() {
    local arch
    arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            print_status "Supported architectures: x86_64, aarch64"
            exit 1
            ;;
    esac
}

# Check if OS is Linux
check_os() {
    if [[ "$(uname -s)" != "Linux" ]]; then
        print_error "This installer only supports Linux"
        exit 1
    fi
}

# Check if systemd is available
check_systemd() {
    if ! command -v systemctl &> /dev/null; then
        print_error "systemd is required but not found"
        exit 1
    fi
}

# Get latest release info
get_latest_release() {
    local api_url="https://api.github.com/repos/$REPO/releases/latest"
    curl -s "$api_url" | grep '"tag_name"' | sed 's/.*"tag_name": "\([^"]*\)".*/\1/'
}

# Download and verify binary
download_binary() {
    local arch=$1
    local version=$2
    local binary_name="tailstream-agent-linux-$arch"
    local download_url="https://github.com/$REPO/releases/download/$version/$binary_name"
    local checksum_url="https://github.com/$REPO/releases/download/$version/checksums.txt"

    print_status "Downloading $binary_name..."

    # Create temporary directory
    local temp_dir
    temp_dir=$(mktemp -d)
    cd "$temp_dir"

    # Download binary and checksums
    if ! curl -L -o "$binary_name" "$download_url"; then
        print_error "Failed to download binary"
        exit 1
    fi

    if ! curl -L -o "checksums.txt" "$checksum_url"; then
        print_warning "Could not download checksums, skipping verification"
    else
        print_status "Verifying checksum..."
        if ! sha256sum -c --ignore-missing checksums.txt; then
            print_error "Checksum verification failed"
            exit 1
        fi
        print_success "Checksum verified"
    fi

    # Install binary
    print_status "Installing binary to $INSTALL_DIR/$BINARY_NAME..."
    chmod +x "$binary_name"
    mv "$binary_name" "$INSTALL_DIR/$BINARY_NAME"

    # Cleanup
    cd /
    rm -rf "$temp_dir"
}

# Create user for service
create_user() {
    if ! id "$USER_NAME" &>/dev/null; then
        print_status "Creating user $USER_NAME..."
        useradd --system --no-create-home --shell /bin/false "$USER_NAME"
    fi
}

# Set up log file permissions (ACL preferred, group fallback)
setup_log_permissions() {
    if command -v setfacl &> /dev/null; then
        setup_acl_permissions
    else
        setup_group_permissions
    fi
}

# Set up ACL permissions for log directories
setup_acl_permissions() {
    print_status "Setting up log file permissions with ACL..."

    local log_dirs=("/var/log/nginx" "/var/log/apache2" "/var/log/httpd" "/var/log/caddy")

    for dir in "${log_dirs[@]}"; do
        if [[ -d "$dir" ]]; then
            setfacl -m u:$USER_NAME:rX "$dir" 2>/dev/null || continue
            setfacl -dm u:$USER_NAME:r "$dir" 2>/dev/null || true
            find "$dir" -name "*.log" -type f -exec setfacl -m u:$USER_NAME:r {} \; 2>/dev/null || true
        fi
    done
}

# Set up group-based permissions fallback
setup_group_permissions() {
    print_status "Setting up log file permissions with group access..."
    usermod -a -G adm "$USER_NAME" 2>/dev/null || {
        print_warning "Could not add user to adm group"
    }
}

# Verify log access permissions
verify_log_access() {
    print_status "Verifying log file access..."

    local test_files=(
        "/var/log/nginx/access.log"
        "/var/log/nginx/error.log"
        "/var/log/apache2/access.log"
        "/var/log/apache2/error.log"
        "/var/log/httpd/access_log"
        "/var/log/httpd/error_log"
        "/var/log/caddy/access.log"
    )

    local accessible_logs=0
    local total_existing=0

    for log_file in "${test_files[@]}"; do
        if [[ -f "$log_file" ]]; then
            ((total_existing++))
            if sudo -u "$USER_NAME" test -r "$log_file" 2>/dev/null; then
                ((accessible_logs++))
                print_success "✓ Can read $log_file"
            else
                print_warning "✗ Cannot read $log_file"
            fi
        fi
    done

    if [[ $total_existing -eq 0 ]]; then
        print_warning "No common log files found - permissions will be set when logs are created"
    elif [[ $accessible_logs -eq $total_existing ]]; then
        print_success "All existing log files are accessible"
    else
        print_warning "Some log files may need manual permission setup"
        print_status "See troubleshooting section in the completion message"
    fi
}

# Create config directory
create_config_dir() {
    print_status "Creating config directory..."
    mkdir -p "$CONFIG_DIR"
    chown root:$USER_NAME "$CONFIG_DIR"
    chmod 750 "$CONFIG_DIR"
}

# Create systemd service
create_service() {
    print_status "Creating systemd service..."

    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Tailstream Agent
Documentation=https://github.com/tailstream-io/tailstream-agent
After=network.target

[Service]
Type=simple
User=$USER_NAME
Group=$USER_NAME
ExecStart=$INSTALL_DIR/$BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=tailstream-agent

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=$CONFIG_DIR
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
}

# Run setup wizard
run_setup() {
    print_status "Running initial setup..."

    # Create a temporary config to run setup as root, then fix ownership
    export TAILSTREAM_CONFIG_PATH="$CONFIG_DIR/agent.yaml"

    if [[ -f "$CONFIG_DIR/agent.yaml" ]]; then
        print_warning "Configuration already exists at $CONFIG_DIR/agent.yaml"
        read -p "Do you want to reconfigure? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return
        fi
    fi

    # Run setup
    "$INSTALL_DIR/$BINARY_NAME"

    # Fix ownership and permissions
    if [[ -f "$CONFIG_DIR/agent.yaml" ]]; then
        chown root:$USER_NAME "$CONFIG_DIR/agent.yaml"
        chmod 640 "$CONFIG_DIR/agent.yaml"
    fi
}

# Start and enable service
start_service() {
    print_status "Starting and enabling tailstream-agent service..."
    systemctl enable tailstream-agent
    systemctl start tailstream-agent

    # Wait a moment and check status
    sleep 2
    if systemctl is-active --quiet tailstream-agent; then
        print_success "Service started successfully"
    else
        print_error "Service failed to start"
        print_status "Check logs with: journalctl -u tailstream-agent -f"
        exit 1
    fi
}

# Show completion message
show_completion() {
    echo
    print_success "🎉 Tailstream Agent installed successfully!"
    echo
    echo "Service Management:"
    echo "  sudo systemctl status tailstream-agent    # Check status"
    echo "  sudo systemctl stop tailstream-agent      # Stop service"
    echo "  sudo systemctl start tailstream-agent     # Start service"
    echo "  sudo systemctl restart tailstream-agent   # Restart service"
    echo
    echo "Logs:"
    echo "  sudo journalctl -u tailstream-agent -f    # Follow logs"
    echo
    echo "Configuration:"
    echo "  sudo nano $CONFIG_DIR/agent.yaml          # Edit config"
    echo
    echo "Log File Permissions:"
    echo "  sudo setfacl -m u:tailstream:r /path/to/log  # Grant access to specific log"
    echo
    echo "To uninstall:"
    echo "  curl https://install.tailstream.io | sudo bash -s -- --uninstall"
    echo
}

# Uninstall function
uninstall() {
    print_status "Uninstalling Tailstream Agent..."

    # Stop and disable service
    if systemctl is-active --quiet tailstream-agent; then
        systemctl stop tailstream-agent
    fi
    if systemctl is-enabled --quiet tailstream-agent; then
        systemctl disable tailstream-agent
    fi

    # Remove files
    rm -f "$SERVICE_FILE"
    rm -f "$INSTALL_DIR/$BINARY_NAME"

    # Remove config (ask first)
    if [[ -d "$CONFIG_DIR" ]]; then
        read -p "Remove configuration directory $CONFIG_DIR? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$CONFIG_DIR"
        fi
    fi

    # Remove user (ask first)
    if id "$USER_NAME" &>/dev/null; then
        read -p "Remove user $USER_NAME? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            userdel "$USER_NAME"
        fi
    fi

    systemctl daemon-reload
    print_success "Uninstalled successfully"
}

# Main installation function
main() {
    echo "🚀 Tailstream Agent Installer"
    echo

    # Handle uninstall
    if [[ "$1" == "--uninstall" ]]; then
        check_root
        uninstall
        exit 0
    fi

    # Pre-flight checks
    check_root
    check_os
    check_systemd

    # Detect system
    local arch
    arch=$(detect_arch)
    print_status "Detected architecture: $arch"

    # Get latest version
    print_status "Getting latest release information..."
    local version
    version=$(get_latest_release)
    if [[ -z "$version" ]]; then
        print_error "Could not determine latest version"
        exit 1
    fi
    print_status "Latest version: $version"

    # Install
    download_binary "$arch" "$version"
    create_user
    create_config_dir
    setup_log_permissions
    create_service
    run_setup
    verify_log_access
    start_service
    show_completion
}

# Run main function with all arguments
main "$@"