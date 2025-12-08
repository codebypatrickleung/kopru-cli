package common

import (
	"strings"
	"sync"
	"testing"
)

func TestMaxNBDDevicesConstant(t *testing.T) {
	if MaxNBDDevices != 8 {
		t.Errorf("MaxNBDDevices = %d; want 8", MaxNBDDevices)
	}
}

func TestMaxPartitionsPerNBDConstant(t *testing.T) {
	if MaxPartitionsPerNBD != 4 {
		t.Errorf("MaxPartitionsPerNBD = %d; want 4", MaxPartitionsPerNBD)
	}
}

func TestGetFreeNBDDeviceFormat(t *testing.T) {
	device, err := GetFreeNBDDevice()
	if err == nil && device != "" {
		if !strings.HasPrefix(device, "/dev/nbd") {
			t.Errorf("GetFreeNBDDevice returned invalid format: %s (expected /dev/nbd prefix)", device)
		}
		deviceNum := strings.TrimPrefix(device, "/dev/nbd")
		if deviceNum == "" {
			t.Errorf("GetFreeNBDDevice returned device without number: %s", device)
		}
	}
}

func TestFindMountablePartitionLogic(t *testing.T) {
	testDevice := "/dev/nbd0"
	_, err := FindMountablePartition(testDevice)
	if err == nil {
		t.Error("Expected error for non-existent device, got nil")
	}
	expectedMsg := "no mountable partition found"
	if err != nil && len(err.Error()) < len(expectedMsg) {
		t.Errorf("Error message too short: %s", err.Error())
	}
}

func TestIsNBDModuleLoadedWithoutModule(t *testing.T) {
	loaded := IsNBDModuleLoaded()
	t.Logf("IsNBDModuleLoaded returned: %v", loaded)
}

func TestGetNBDDeviceFromMountPointEmpty(t *testing.T) {
	device := getNBDDeviceFromMountPoint("")
	if device != "" {
		t.Errorf("Expected empty string for empty mount point, got: %s", device)
	}
}

func TestGetNBDDeviceFromMountPointNonExistent(t *testing.T) {
	device := getNBDDeviceFromMountPoint("/non/existent/mount/point")
	if device != "" {
		t.Errorf("Expected empty string for non-existent mount point, got: %s", device)
	}
}

func TestIsNBDDeviceConnectedNonExistent(t *testing.T) {
	connected := isNBDDeviceConnected("/dev/nonexistent")
	if connected {
		t.Error("Expected false for non-existent device")
	}
}

// TestNBDMutexProtection verifies that the mutex protects concurrent access
// to NBD device allocation and prevents race conditions.
func TestNBDMutexProtection(t *testing.T) {
	// This test verifies that nbdMutex is properly defined and accessible
	// The actual race condition protection is tested during runtime with
	// concurrent goroutines in the data disk import workflow
	
	// Test that multiple goroutines can safely call GetFreeNBDDevice
	// without causing data races (this would be caught by go test -race)
	var wg sync.WaitGroup
	deviceChan := make(chan string, 3)
	
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			device, err := GetFreeNBDDevice()
			if err == nil && device != "" {
				deviceChan <- device
			}
		}()
	}
	
	wg.Wait()
	close(deviceChan)
	
	// Verify that devices returned (if any) have the correct format
	for device := range deviceChan {
		if !strings.HasPrefix(device, "/dev/nbd") {
			t.Errorf("GetFreeNBDDevice returned invalid format in concurrent call: %s", device)
		}
	}
}
