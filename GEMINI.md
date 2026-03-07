# Gemini CLI Interaction Guidelines

This document outlines guidelines for effective interaction with the Gemini CLI agent in the `cask` project.

## Project Overview

`cask` is a distributed key-value store written in Go.

## Project-Specific Commands

You can use the following `make` commands to build and test the project:

*   `make cask`: Build the `cask` binary.
*   `make test`: Run the test suite.
*   `make cluster`: Run a local cluster for testing.

## Example Interactions

*   **To read a file:** `read_file(file_path='cask.go')`
*   **To search for a function:** `search_file_content(pattern='func handleSet', include='*.go')`
*   **To run the tests:** `run_shell_command(command='make test')`

## Project guidelines

- Run `make test` before beginning work to ensure that the code is starting in a clean state.
- Run `make test` and `make lint` when you are done with all changes and fix any pending issues
- Always develop with strict red/green TDD unless told otherwise.


