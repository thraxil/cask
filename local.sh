#!/bin/sh
sudo systemctl stop cask-sata1
sudo systemctl stop cask-sata2
sudo systemctl stop cask-sata3
sudo systemctl stop cask-sata4
sudo systemctl stop cask-sata5
sudo systemctl stop cask-sata7
sudo systemctl stop cask-sata8
sudo systemctl stop cask-sata9
sudo systemctl stop cask-sata10
sudo systemctl stop cask-sata11
sudo systemctl stop cask-sata12
make
sudo make install
sudo systemctl start cask-sata1
sudo systemctl start cask-sata2
sudo systemctl start cask-sata3
sudo systemctl start cask-sata4
sudo systemctl start cask-sata5
sudo systemctl start cask-sata7
sudo systemctl start cask-sata8
sudo systemctl start cask-sata9
sudo systemctl start cask-sata10
sudo systemctl start cask-sata11
sudo systemctl start cask-sata12
