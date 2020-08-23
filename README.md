# s3s

[![Actions Status](https://github.com/dlampsi/s3s/workflows/default/badge.svg)](https://github.com/dlampsi/s3s/actions)

App for synchronize objects from S3 to host.

## Setup

### Binary file
You can download binary file releases [here](https://github.com/dlampsi/s3s/releases).

Install commands example for version `0.0.1` on linux OS:
```bash
wget https://github.com/dlampsi/s3s/releases/download/0.0.1/s3s_0.0.1_linux_amd64.zip
unzip s3s_0.0.1_linux_amd64.zip
mv s3s_0.0.1_linux_amd64 /usr/local/bin/s3s
chmod +x /usr/local/bin/s3s
```

### Docker image

You can run s3s via Docker by using oficial [image](https://hub.docker.com/repository/docker/dlampsi/s3s).

```bash
# Pull image
docker pull dlampsi/s3s:latest
# Run commands
docker run --name s3s -p 8085:8085 s3s:latest pull s3://bucket/folder/ /local/folder
```

### Add cmd auto-completion

To add s3s commands auto-competion use `completion` command:

```bash
s3s completion -h
```

Example setup completion for bash on linux:

```bash
s3s completion bash > /etc/bash_completion.d/s3s
```

## Usage

```bash
s3s pull s3://bucket/folder/ /local/folder
```