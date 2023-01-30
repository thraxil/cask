{ pkgs ? import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/06278c77b5d162e62df170fec307e83f1812d94b.tar.gz") {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.go_1_19
    pkgs.gcc
    pkgs.libcap
    pkgs.python310
  ];

  shellHook = ''
  '';

  MY_ENVIRONMENT_VARIABLE = "world";
}
