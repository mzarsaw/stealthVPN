# StealthVPN Android Client Integration

This document explains how to integrate the StealthVPN Android client library into your Android application.

## Prerequisites

- Android Studio 4.0+
- Android SDK API 21+ (Android 5.0)
- Go 1.21+ with gomobile installed
- Android VPN permission in your app

## Building the Android Library

1. Install gomobile:
```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

2. Build the Android library:
```bash
cd client/android
gomobile bind -target=android -o stealthvpn.aar .
```

## Android App Integration

### 1. Add the Library

Copy `stealthvpn.aar` to your Android project's `app/libs/` directory.

In your `app/build.gradle`:
```gradle
android {
    compileSdk 34
    
    defaultConfig {
        minSdk 21
        targetSdk 34
    }
}

dependencies {
    implementation files('libs/stealthvpn.aar')
    implementation 'androidx.appcompat:appcompat:1.6.1'
    implementation 'com.google.android.material:material:1.9.0'
}
```

### 2. Add Permissions

In your `AndroidManifest.xml`:
```xml
<uses-permission android:name="android.permission.INTERNET" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.BIND_VPN_SERVICE" />

<service
    android:name=".StealthVPNService"
    android:permission="android.permission.BIND_VPN_SERVICE"
    android:exported="false">
    <intent-filter>
        <action android:name="android.net.VpnService" />
    </intent-filter>
</service>
```

### 3. Create VPN Service

Create `StealthVPNService.java`:
```java
package com.yourapp.stealthvpn;

import android.content.Intent;
import android.net.VpnService;
import android.os.ParcelFileDescriptor;
import android.util.Log;

import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.nio.ByteBuffer;

import main.AndroidVPNClient;
import main.VPNService;

public class StealthVPNService extends VpnService implements VPNService {
    private static final String TAG = "StealthVPNService";
    private ParcelFileDescriptor vpnInterface;
    private FileInputStream inputStream;
    private FileOutputStream outputStream;
    private AndroidVPNClient vpnClient;
    private Thread packetReaderThread;
    private volatile boolean isRunning = false;

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String configJson = intent.getStringExtra("config");
        if (configJson != null) {
            startVPN(configJson);
        }
        return START_STICKY;
    }

    private void startVPN(String configJson) {
        try {
            // Create VPN client
            vpnClient = new AndroidVPNClient(configJson, this);
            
            // Start VPN connection
            vpnClient.startVPN();
            
            // Start packet reading thread
            startPacketReader();
            
        } catch (Exception e) {
            Log.e(TAG, "Failed to start VPN", e);
        }
    }

    @Override
    public boolean createTunInterface(String ip, String[] dns) {
        try {
            Builder builder = new Builder();
            builder.setMtu(1500);
            builder.addAddress(ip, 24);
            builder.addRoute("0.0.0.0", 0);
            
            for (String dnsServer : dns) {
                builder.addDnsServer(dnsServer);
            }
            
            builder.setSession("StealthVPN");
            vpnInterface = builder.establish();
            
            if (vpnInterface != null) {
                inputStream = new FileInputStream(vpnInterface.getFileDescriptor());
                outputStream = new FileOutputStream(vpnInterface.getFileDescriptor());
                isRunning = true;
                return true;
            }
        } catch (Exception e) {
            Log.e(TAG, "Failed to create TUN interface", e);
        }
        return false;
    }

    @Override
    public boolean writePacket(byte[] data) {
        try {
            if (outputStream != null) {
                outputStream.write(data);
                return true;
            }
        } catch (Exception e) {
            Log.e(TAG, "Failed to write packet", e);
        }
        return false;
    }

    @Override
    public byte[] readPacket() {
        try {
            if (inputStream != null) {
                byte[] buffer = new byte[1500];
                int length = inputStream.read(buffer);
                if (length > 0) {
                    byte[] packet = new byte[length];
                    System.arraycopy(buffer, 0, packet, 0, length);
                    return packet;
                }
            }
        } catch (Exception e) {
            Log.e(TAG, "Failed to read packet", e);
        }
        return null;
    }

    @Override
    public boolean closeTunInterface() {
        isRunning = false;
        try {
            if (vpnInterface != null) {
                vpnInterface.close();
                vpnInterface = null;
            }
            if (inputStream != null) {
                inputStream.close();
                inputStream = null;
            }
            if (outputStream != null) {
                outputStream.close();
                outputStream = null;
            }
            return true;
        } catch (Exception e) {
            Log.e(TAG, "Failed to close TUN interface", e);
        }
        return false;
    }

    @Override
    public boolean isConnected() {
        return isRunning && vpnInterface != null;
    }

    private void startPacketReader() {
        packetReaderThread = new Thread(() -> {
            while (isRunning) {
                try {
                    byte[] packet = readPacket();
                    if (packet != null && vpnClient != null) {
                        // Process packet through VPN client
                        // This would typically be handled by the Go client
                    }
                    Thread.sleep(1);
                } catch (Exception e) {
                    Log.e(TAG, "Error in packet reader", e);
                }
            }
        });
        packetReaderThread.start();
    }

    @Override
    public void onDestroy() {
        if (vpnClient != null) {
            vpnClient.stopVPN();
        }
        closeTunInterface();
        super.onDestroy();
    }
}
```

### 4. Main Activity Integration

Create `MainActivity.java`:
```java
package com.yourapp.stealthvpn;

import android.app.Activity;
import android.content.Intent;
import android.net.VpnService;
import android.os.Bundle;
import android.widget.Button;
import android.widget.Toast;

public class MainActivity extends Activity {
    private static final int VPN_REQUEST_CODE = 1;
    private static final String CONFIG_JSON = "{"
        + "\"server_url\": \"wss://YOUR_SERVER_IP:443/ws\","
        + "\"pre_shared_key\": \"YOUR_PRE_SHARED_KEY_HERE\","
        + "\"dns_servers\": [\"8.8.8.8\", \"8.8.4.4\"],"
        + "\"local_ip\": \"10.8.0.3\","
        + "\"auto_connect\": true,"
        + "\"reconnect_delay\": 5,"
        + "\"health_check_interval\": 30,"
        + "\"fake_domain_name\": \"api.cloudsync-enterprise.com\""
        + "}";

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        Button connectButton = findViewById(R.id.connect_button);
        Button disconnectButton = findViewById(R.id.disconnect_button);

        connectButton.setOnClickListener(v -> requestVpnPermission());
        disconnectButton.setOnClickListener(v -> stopVpnService());
    }

    private void requestVpnPermission() {
        Intent intent = VpnService.prepare(this);
        if (intent != null) {
            startActivityForResult(intent, VPN_REQUEST_CODE);
        } else {
            startVpnService();
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        if (requestCode == VPN_REQUEST_CODE && resultCode == RESULT_OK) {
            startVpnService();
        } else {
            Toast.makeText(this, "VPN permission denied", Toast.LENGTH_SHORT).show();
        }
        super.onActivityResult(requestCode, resultCode, data);
    }

    private void startVpnService() {
        Intent intent = new Intent(this, StealthVPNService.class);
        intent.putExtra("config", CONFIG_JSON);
        startService(intent);
        Toast.makeText(this, "VPN Started", Toast.LENGTH_SHORT).show();
    }

    private void stopVpnService() {
        Intent intent = new Intent(this, StealthVPNService.class);
        stopService(intent);
        Toast.makeText(this, "VPN Stopped", Toast.LENGTH_SHORT).show();
    }
}
```

### 5. Layout File

Create `res/layout/activity_main.xml`:
```xml
<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:orientation="vertical"
    android:padding="16dp">

    <TextView
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:text="StealthVPN"
        android:textSize="24sp"
        android:textAlignment="center"
        android:layout_marginBottom="32dp" />

    <Button
        android:id="@+id/connect_button"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:text="Connect VPN"
        android:layout_marginBottom="16dp" />

    <Button
        android:id="@+id/disconnect_button"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:text="Disconnect VPN" />

</LinearLayout>
```

## Configuration

Update the `CONFIG_JSON` in `MainActivity.java` with:
- Your server's IP address
- The pre-shared key from server setup
- Appropriate DNS servers for your region

## Testing

1. Build and install the app
2. Start your StealthVPN server
3. Tap "Connect VPN" in the app
4. Grant VPN permissions when prompted
5. Check connection in the app logs

## Troubleshooting

- **Build errors**: Ensure gomobile is properly installed
- **VPN permission denied**: Check manifest permissions
- **Connection fails**: Verify server configuration
- **No internet**: Check DNS settings and routing

## Security Notes

- Use a real TLS certificate in production
- Store configuration securely (not hardcoded)
- Implement proper error handling
- Consider using certificate pinning for additional security

For more information, see the main project documentation. 