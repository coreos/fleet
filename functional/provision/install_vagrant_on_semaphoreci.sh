#!/bin/bash -e

echo "Installing Vagrant and VirtualBox..."
echo "deb http://download.virtualbox.org/virtualbox/debian $(lsb_release -sc) contrib" | sudo tee /etc/apt/sources.list.d/virtualbox.list
wget -q https://www.virtualbox.org/download/oracle_vbox.asc -O- | sudo apt-key add -
sudo apt-get update -qq
sudo apt-get install -qq -y build-essential dkms nfs-kernel-server linux-headers-`uname -r`
wget -P /tmp/ https://releases.hashicorp.com/vagrant/1.8.1/vagrant_1.8.1_x86_64.deb
sudo dpkg -i /tmp/vagrant_1.8.1_x86_64.deb
sudo apt-get install -qq -y VirtualBox-4.3
