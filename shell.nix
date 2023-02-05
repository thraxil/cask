{ pkgs ? import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/0591d6b57bfeb55dfeec99a671843337bc2c3323.tar.gz") {} }:

pkgs.mkShell {
  buildInputs = [
    pkgs.go_1_20
    pkgs.gcc
    pkgs.libcap
    pkgs.python310
  ];

  shellHook = ''
  '';

  MY_ENVIRONMENT_VARIABLE = "world";
}
