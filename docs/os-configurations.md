# OS Configurations

All OS-specific configurations used by the Kopru CLI during image migration are written as bash script files located in the `scripts/os-config/` directory.

## Benefits of Using Bash Scripts

There are several benefits to using bash scripts for OS configurations:

- **Easier maintenance**: Bash scripts are easier to modify and test independently.
- **Better separation of concerns**: Any changes to the VM are isolated from the Go application.
- **Flexibility**: New OS configurations can be added by simply editing the scripts without changing the Go code.
- **Transparency**: Configuration changes are more visible and auditable.

## Configuration Scripts Location

You can find all OS configuration scripts in the `scripts/os-config/` directory of the Kopru CLI repository.
