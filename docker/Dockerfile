FROM debian:stable-slim

# Install essential packages
RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends \
        build-essential \
        coreutils \
        binutils \
        diffutils \
        libtool \
        perl \
        git \
        curl \
        ncat \
        autoconf \
        automake \
        python3 \
        openjdk-17-jdk \
        gcc \
        clang \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# Set JAVA_HOME environment variable
ENV JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64

# Verify installations
RUN java -version && \
    python3 --version && \
    gcc --version && \
    clang --version

COPY entrypoint.py /
COPY exit.sh /
ENTRYPOINT ["python3", "/entrypoint.py"]
CMD ["python3", "/entrypoint.py"]
