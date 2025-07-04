# ETW Buffer Monitor

A real-time monitoring tool for Event Tracing for Windows (ETW) session buffers, built with Go and featuring a beautiful terminal user interface powered by Bubble Tea.

![ETW Buffer Monitor Screenshot](https://img.shields.io/badge/Platform-Windows-blue?style=for-the-badge&logo=windows)
![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=for-the-badge&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)

## 🚀 Features

- **Real-time monitoring** of all active ETW sessions
- **Beautiful terminal UI** with smooth updates (no screen flickering)
- **Color-coded status indicators**:
  - 🔴 **Red**: Sessions with lost events (critical)
  - 🟠 **Orange**: High buffer utilization (>80%)
  - 🟢 **Green**: Sessions with recent changes
  - ⚪ **White**: Normal sessions
- **Compact side-by-side layout** for summary and warnings
- **Change highlighting** to spot active sessions
- **CSV export** functionality
- **Configurable refresh intervals**
- **One-time snapshots** for quick checks

## 📋 Requirements

- **Windows OS** (uses Windows ETW APIs)
- **Administrator privileges** (required to access ETW sessions)
- **Go 1.23+** (for building from source)

## 🔧 Installation

### Build from Source
```powershell
# Clone or download the source code
# Navigate to the project directory
cd ETWtop

# Install dependencies
go mod tidy

# Build the executable
go build .
```

## 🎯 Usage

### Basic Commands

```powershell
# Start continuous monitoring (1-second refresh by default)
.\ETWtop.exe

# Show current stats once and exit
.\ETWtop.exe -once

# Monitor with custom refresh interval (5 seconds)
.\ETWtop.exe -interval 5

# Export current stats to CSV
.\ETWtop.exe -export stats.csv

# Show help
.\ETWtop.exe -help
```

### Command Line Options

| Option | Description | Default |
|--------|-------------|---------|
| `-once` | Show buffer info once and exit | Continuous monitoring |
| `-export [filename]` | Export to CSV file | `etw_buffer_stats.csv` |
| `-interval [seconds]` | Monitoring refresh interval | `1` second |
| `-help` | Show help message | - |

### Interactive Controls

During continuous monitoring:
- **`q`** or **`Ctrl+C`** - Quit the application

## 📊 Display Information

The monitor shows the following information for each ETW session:

| Column | Description |
|--------|-------------|
| **Session Name** | Name of the ETW session |
| **Buffer(KB)** | Size of each buffer in kilobytes |
| **Min** | Minimum number of buffers |
| **Max** | Maximum number of buffers |
| **Current** | Current number of allocated buffers |
| **Free** | Number of free buffers |
| **Written** | Total buffers written |
| **Lost** | Number of lost events |
| **Util%** | Buffer utilization percentage |
| **Memory(MB)** | Total memory usage |

### Summary Box
- **Total Sessions**: Number of active ETW sessions
- **Total Memory**: Combined memory usage of all sessions
- **Avg Utilization**: Average buffer utilization across sessions
- **Total Events Lost**: Total events lost across all sessions

### Warning Box
Displays alerts for:
- Sessions with high buffer utilization (>80%)
- Sessions with lost events

## 🎨 Visual Features

- **Rounded border boxes** for clean presentation
- **Color-coded warnings** for quick problem identification
- **Real-time change highlighting** for active sessions
- **Smooth updates** without screen clearing or flickering
- **Professional terminal dashboard** appearance

## 📁 CSV Export Format

When exporting to CSV, the following columns are included:
- Timestamp
- SessionName
- BufferSize_KB
- MinBuffers, MaxBuffers
- NumberOfBuffers, FreeBuffers
- BuffersWritten, EventsLost, RealTimeBuffersLost
- UtilizationPercent, TotalMemory_MB
- LogFileName

## ⚠️ Important Notes

1. **Administrator Rights Required**: This tool requires administrator privileges to access ETW session information.

2. **Windows Only**: Uses Windows-specific ETW APIs and is not compatible with other operating systems.

3. **Performance Impact**: Monitoring has minimal performance impact, but very frequent updates (sub-second intervals) may increase CPU usage slightly.

## 🔍 Troubleshooting

### Common Issues

**"Access Denied" or no sessions showing:**
- Ensure you're running as Administrator
- Some ETW sessions may only be visible to SYSTEM account

**Build errors:**
- Ensure Go 1.23+ is installed
- Run `go mod tidy` to resolve dependencies
- Check that you're on Windows (required for ETW APIs)

**High CPU usage:**
- Increase the refresh interval: `.\ETWtop.exe -interval 5`
- Use `-once` for one-time checks instead of continuous monitoring

## 🛠️ Technical Details

### Dependencies
- **Bubble Tea** - Terminal user interface framework
- **Lipgloss** - Styling and layout for terminal output
- **Windows ETW APIs** - Native Windows event tracing functionality

### Architecture
- Built using the Elm architecture pattern via Bubble Tea
- Efficient state management with change detection
- Direct Windows API calls for ETW session enumeration
- Minimal memory footprint with optimized rendering


## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🔗 Related Tools

- **Windows Performance Toolkit (WPT)** - Official Microsoft ETW tools, primarily xperf.exe
- **logman.exe** - Built-in Windows ETW management utility
