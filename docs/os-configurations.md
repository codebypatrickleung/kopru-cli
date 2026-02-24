# OS Configurations

Kopru CLI uses bash script files for all operating system (OS)-specific configurations during image migration. These scripts are located in the `scripts/os-config/` directory.

## Benefits of Using Bash Scripts

Using bash scripts for OS configurations provides several advantages:

- **Easier maintenance:** Scripts can be modified and tested independently.
- **Separation of concerns:** Changes to the virtual machine are isolated from the Go application.
- **Flexibility:** New OS configurations can be added by editing scripts without changing Go source code.
- **Transparency:** Configuration changes are clear, auditable, and easily tracked.

## Location of Configuration Scripts

All OS configuration scripts are located in the `scripts/os-config/` directory of the Kopru CLI repository.
