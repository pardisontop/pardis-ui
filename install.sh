#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Check OS and set release variable
if [[ -f /etc/os-release ]]; then
    source /etc/os-release
    release=$ID
elif [[ -f /usr/lib/os-release ]]; then
    source /usr/lib/os-release
    release=$ID
else
    echo "Failed to check the system OS, please contact the author!" >&2
    exit 1
fi
echo "The OS release is: $release"

arch() {
    case "$(uname -m)" in
    x86_64 | x64 | amd64) echo 'amd64' ;;
    i*86 | x86) echo '386' ;;
    armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
    armv7* | armv7 | arm) echo 'armv7' ;;
    armv6* | armv6) echo 'armv6' ;;
    armv5* | armv5) echo 'armv5' ;;
    s390x) echo 's390x' ;;
    *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

echo "arch: $(arch)"

install_dependencies() {
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -y -q wget curl tar tzdata cron
        ;;
    centos | almalinux | rocky | ol)
        yum -y update && yum install -y -q wget curl tar tzdata cronie
        ;;
    fedora | amzn)
        dnf -y update && dnf install -y -q wget curl tar tzdata cronie
        ;;
    arch | manjaro | parch)
        pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata cronie
        ;;
    opensuse-tumbleweed)
        zypper refresh && zypper -q install -y wget curl tar timezone cron
        ;;
    *)
        apt-get update && apt install -y -q wget curl tar tzdata cron
        ;;
    esac
}

gen_random_string() {
    local length="$1"
    local random_string=$(LC_ALL=C tr -dc 'a-zA-Z0-9' </dev/urandom | fold -w "$length" | head -n 1)
    echo "$random_string"
}

config_after_install() {
    local existing_username=$(/usr/local/pardis-ui/pardis-ui setting -show true | grep -Eo 'username: .+' | awk '{print $2}')
    local existing_password=$(/usr/local/pardis-ui/pardis-ui setting -show true | grep -Eo 'password: .+' | awk '{print $2}')
    local existing_webBasePath=$(/usr/local/pardis-ui/pardis-ui setting -show true | grep -Eo 'webBasePath: .+' | awk '{print $2}')

    if [[ ${#existing_webBasePath} -lt 4 ]]; then
        if [[ "$existing_username" == "admin" && "$existing_password" == "admin" ]]; then
            local config_webBasePath=$(gen_random_string 15)
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            read -p "Would you like to customize the Panel Port settings? (If not, random port will be applied) [y/n]: " config_confirm
            if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
                read -p "Please set up the panel port: " config_port
                echo -e "${yellow}Your Panel Port is: ${config_port}${plain}"
            else
                local config_port=$(shuf -i 1024-62000 -n 1)
                echo -e "${yellow}Generated random port: ${config_port}${plain}"
            fi

            /usr/local/pardis-ui/pardis-ui setting -username "${config_username}" -password "${config_password}" -port "${config_port}" -webBasePath "${config_webBasePath}"
            echo -e "This is a fresh installation, generating random login info for security concerns:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "${green}Port: ${config_port}${plain}"
            echo -e "${green}WebBasePath: ${config_webBasePath}${plain}"
            echo -e "###############################################"
            echo -e "${yellow}If you forgot your login info, you can type 'pardis-ui settings' to check${plain}"
        else
            local config_webBasePath=$(gen_random_string 15)
            echo -e "${yellow}WebBasePath is missing or too short. Generating a new one...${plain}"
            /usr/local/pardis-ui/pardis-ui setting -webBasePath "${config_webBasePath}"
            echo -e "${green}New WebBasePath: ${config_webBasePath}${plain}"
        fi
    else
        if [[ "$existing_username" == "admin" && "$existing_password" == "admin" ]]; then
            local config_username=$(gen_random_string 10)
            local config_password=$(gen_random_string 10)

            echo -e "${yellow}Default credentials detected. Security update required...${plain}"
            /usr/local/pardis-ui/pardis-ui setting -username "${config_username}" -password "${config_password}"
            echo -e "Generated new random login credentials:"
            echo -e "###############################################"
            echo -e "${green}Username: ${config_username}${plain}"
            echo -e "${green}Password: ${config_password}${plain}"
            echo -e "###############################################"
            echo -e "${yellow}If you forgot your login info, you can type 'pardis-ui settings' to check${plain}"
        else
            echo -e "${green}Username, Password, and WebBasePath are properly set. Exiting...${plain}"
        fi
    fi

    /usr/local/pardis-ui/pardis-ui migrate
}

install_pardis-ui() {
    # checks if the installation backup dir exist. if existed then ask user if they want to restore it else continue installation.
    if [[ -e /usr/local/pardis-ui-backup/ ]]; then
        read -p "Failed installation detected. Do you want to restore previously installed version? [y/n]? ": restore_confirm
        if [[ "${restore_confirm}" == "y" || "${restore_confirm}" == "Y" ]]; then
            systemctl stop pardis-ui
            mv /usr/local/pardis-ui-backup/pardis-ui.db /etc/pardis-ui/ -f
            mv /usr/local/pardis-ui-backup/ /usr/local/pardis-ui/ -f
            systemctl start pardis-ui
            echo -e "${green}previous installed pardis-ui restored successfully${plain}, it is up and running now..."
            exit 0
        else
            echo -e "Continuing installing pardis-ui ..."
        fi
    fi

    cd /usr/local/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/alireza0/pardis-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            echo -e "${red}Failed to fetch pardis-ui version, it maybe due to Github API restrictions, please try it later${plain}"
            exit 1
        fi
        echo -e "Got pardis-ui latest version: ${last_version}, beginning the installation..."
        wget -N --no-check-certificate -O /usr/local/pardis-ui-linux-$(arch).tar.gz https://github.com/alireza0/pardis-ui/releases/download/${last_version}/pardis-ui-linux-$(arch).tar.gz
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Downloading pardis-ui failed, please be sure that your server can access Github ${plain}"
            exit 1
        fi
    else
        last_version=$1
        url="https://github.com/alireza0/pardis-ui/releases/download/${last_version}/pardis-ui-linux-$(arch).tar.gz"
        echo -e "Beginning to install pardis-ui $1"
        wget -N --no-check-certificate -O /usr/local/pardis-ui-linux-$(arch).tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}download pardis-ui $1 failed,please check the version exists${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/pardis-ui/ ]]; then
        systemctl stop pardis-ui
        mv /usr/local/pardis-ui/ /usr/local/pardis-ui-backup/ -f
        cp /etc/pardis-ui/pardis-ui.db /usr/local/pardis-ui-backup/ -f
    fi

    tar zxvf pardis-ui-linux-$(arch).tar.gz
    rm pardis-ui-linux-$(arch).tar.gz -f
    cd pardis-ui
    chmod +x pardis-ui

    # Check the system's architecture and rename the file accordingly
    if [[ $(arch) == "armv7" ]]; then
        mv bin/xray-linux-$(arch) bin/xray-linux-arm
        chmod +x bin/xray-linux-arm
    fi
    chmod +x pardis-ui bin/xray-linux-$(arch)
    cp -f pardis-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/pardis-ui https://raw.githubusercontent.com/alireza0/pardis-ui/main/pardis-ui.sh
    chmod +x /usr/local/pardis-ui/pardis-ui.sh
    chmod +x /usr/bin/pardis-ui
    config_after_install
    rm /usr/local/pardis-ui-backup/ -rf
    #echo -e "If it is a new installation, the default web port is ${green}54321${plain}, The username and password are ${green}admin${plain} by default"
    #echo -e "Please make sure that this port is not occupied by other procedures,${yellow} And make sure that port 54321 has been released${plain}"
    #    echo -e "If you want to modify the 54321 to other ports and enter the pardis-ui command to modify it, you must also ensure that the port you modify is also released"
    #echo -e ""
    #echo -e "If it is updated panel, access the panel in your previous way"
    #echo -e ""
    systemctl daemon-reload
    systemctl enable pardis-ui
    systemctl start pardis-ui
    echo -e "${green}pardis-ui ${last_version}${plain} installation finished, it is up and running now..."
    echo -e ""
    echo -e "You may access the Panel with following URL(s):${yellow}"
    /usr/local/pardis-ui/pardis-ui uri
    echo -e "${plain}"
    echo "pardis-ui Control Menu Usage"
    echo "------------------------------------------"
    echo "SUBCOMMANDS:"
    echo "pardis-ui              - Admin Management Script"
    echo "pardis-ui start        - Start"
    echo "pardis-ui stop         - Stop"
    echo "pardis-ui restart      - Restart"
    echo "pardis-ui status       - Current Status"
    echo "pardis-ui settings     - Current Settings"
    echo "pardis-ui enable       - Enable Autostart on OS Startup"
    echo "pardis-ui disable      - Disable Autostart on OS Startup"
    echo "pardis-ui log          - Check Logs"
    echo "pardis-ui update       - Update"
    echo "pardis-ui install      - Install"
    echo "pardis-ui uninstall    - Uninstall"
    echo "pardis-ui help         - Control Menu Usage"
    echo "------------------------------------------"
}

echo -e "${green}Running...${plain}"
install_dependencies
install_pardis-ui $1
