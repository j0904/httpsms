FROM eclipse-temurin:21-jdk

ENV ANDROID_HOME=/opt/android-sdk
ENV PATH=$PATH:$ANDROID_HOME/cmdline-tools/latest/bin:$ANDROID_HOME/platform-tools:$ANDROID_HOME/build-tools/36.0.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends unzip wget git \
    && rm -rf /var/lib/apt/lists/*

# Android command-line tools (rev 12.0)
RUN mkdir -p $ANDROID_HOME/cmdline-tools \
    && wget -q https://dl.google.com/android/repository/commandlinetools-linux-13114758_latest.zip -O /tmp/cmdline.zip \
    && unzip -q /tmp/cmdline.zip -d $ANDROID_HOME/cmdline-tools \
    && mv $ANDROID_HOME/cmdline-tools/cmdline-tools $ANDROID_HOME/cmdline-tools/latest \
    && rm /tmp/cmdline.zip

# Accept licenses and install platform 36 + build-tools 36.0.0
RUN yes | sdkmanager --licenses > /dev/null 2>&1 \
    && sdkmanager "platforms;android-36" "build-tools;36.0.0" platform-tools > /dev/null

WORKDIR /app
COPY android/ /app/
COPY .git/ /app/.git/

RUN chmod +x gradlew \
    && ./gradlew assembleRelease --no-daemon
