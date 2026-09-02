import Cocoa

// 双向 socket 协议：
//   path_in  (CLI arg 1): helper 监听，主进程向其发送命令
//   path_out (CLI arg 2): 主进程监听，helper 向其发送菜单动作
// 帧格式：4 字节大端长度 + payload
// 命令：
//   主进程 -> helper: 'S' + PNG 字节（可选：用于覆盖默认图标）
//   helper   -> 主进程: 'O' 或 'Q'

final class TrayApp: NSObject, NSApplicationDelegate {
    var statusItem: NSStatusItem!
    var pathIn: String = ""
    var pathOut: String = ""
    var fdIn: Int32 = -1
    var srcIn: DispatchSourceRead!  // 必须保持引用，否则 dispatch 源会被立即释放

    func applicationDidFinishLaunching(_ notification: Notification) {
        let args = CommandLine.arguments
        guard args.count > 2 else {
            FileHandle.standardError.write("usage: tray <path_in> <path_out>\n".data(using: .utf8)!)
            exit(1)
        }
        pathIn = args[1]
        pathOut = args[2]

        NSApp.setActivationPolicy(.accessory)

        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        if let btn = statusItem.button,
           let img = NSImage(systemSymbolName: "doc.on.clipboard", accessibilityDescription: "ClipSync") {
            img.size = NSSize(width: 18, height: 18)
            btn.image = img
        }

        let menu = NSMenu()
        let open = NSMenuItem(title: "打开主窗口", action: #selector(onOpen), keyEquivalent: "")
        open.target = self
        menu.addItem(open)
        menu.addItem(NSMenuItem.separator())
        let quit = NSMenuItem(title: "退出 ClipSync", action: #selector(onQuit), keyEquivalent: "")
        quit.target = self
        menu.addItem(quit)
        statusItem.menu = menu

        unlink(pathIn)
        fdIn = socket(AF_UNIX, SOCK_STREAM, 0)
        if fdIn < 0 { exit(1) }
        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        let pb = Array(pathIn.utf8CString)
        if pb.count > MemoryLayout.size(ofValue: addr.sun_path) { exit(1) }
        withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
            ptr.withMemoryRebound(to: CChar.self, capacity: pb.count) { reb in
                for (i, b) in pb.enumerated() { reb[i] = b }
            }
        }
        let br = withUnsafePointer(to: &addr) { p -> Int32 in
            p.withMemoryRebound(to: sockaddr.self, capacity: 1) { sp in
                Darwin.bind(fdIn, sp, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        if br < 0 { exit(1) }
        chmod(pathIn, 0o600)
        if listen(fdIn, 5) < 0 { exit(1) }

        let q = DispatchQueue(label: "tray.in")
        srcIn = DispatchSource.makeReadSource(fileDescriptor: fdIn, queue: q)
        srcIn.setEventHandler { [weak self] in self?.acceptIn() }
        srcIn.resume()
    }

    func acceptIn() {
        var ca = sockaddr()
        var cl = socklen_t(MemoryLayout<sockaddr>.size)
        let c = accept(fdIn, &ca, &cl)
        if c < 0 { return }
        DispatchQueue.global(qos: .userInitiated).async { [weak self] in
            self?.serveIn(fd: c)
        }
    }

    func serveIn(fd: Int32) {
        var buf = Data()
        var tmp = [UInt8](repeating: 0, count: 4096)
        while true {
            let n = read(fd, &tmp, 4096)
            if n <= 0 { break }
            buf.append(contentsOf: tmp[0..<n])
            while buf.count >= 4 {
                let len = (UInt32(buf[0]) << 24) | (UInt32(buf[1]) << 16) | (UInt32(buf[2]) << 8) | UInt32(buf[3])
                let total = 4 + Int(len)
                if buf.count < total { break }
                let p = buf.subdata(in: 4..<total)
                buf.removeSubrange(0..<total)
                DispatchQueue.main.async { [weak self] in self?.onCmd(p) }
            }
        }
        close(fd)
    }

    func onCmd(_ payload: Data) {
        guard let first = payload.first, first == 0x53 else { return } // 'S'
        let png = payload.subdata(in: 1..<payload.count)
        if let img = NSImage(data: png) {
            img.size = NSSize(width: 18, height: 18)
            statusItem.button?.image = img
            statusItem.button?.title = ""
        }
    }

    @objc func onOpen() { sendOut("O") }
    @objc func onQuit() {
        sendOut("Q")
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.5) { NSApp.terminate(nil) }
    }

    func sendOut(_ s: String) {
        let fd = socket(AF_UNIX, SOCK_STREAM, 0)
        guard fd >= 0 else { return }
        var addr = sockaddr_un()
        addr.sun_family = sa_family_t(AF_UNIX)
        let pb = Array(pathOut.utf8CString)
        if pb.count > MemoryLayout.size(ofValue: addr.sun_path) { close(fd); return }
        withUnsafeMutablePointer(to: &addr.sun_path) { ptr in
            ptr.withMemoryRebound(to: CChar.self, capacity: pb.count) { reb in
                for (i, b) in pb.enumerated() { reb[i] = b }
            }
        }
        let r = withUnsafePointer(to: &addr) { p -> Int32 in
            p.withMemoryRebound(to: sockaddr.self, capacity: 1) { sp in
                Darwin.connect(fd, sp, socklen_t(MemoryLayout<sockaddr_un>.size))
            }
        }
        if r < 0 { close(fd); return }
        let bytes = Array(s.utf8)
        var hdr = [UInt8]()
        let len = UInt32(bytes.count)
        hdr.append(UInt8((len >> 24) & 0xff))
        hdr.append(UInt8((len >> 16) & 0xff))
        hdr.append(UInt8((len >> 8) & 0xff))
        hdr.append(UInt8(len & 0xff))
        hdr.append(contentsOf: bytes)
        _ = hdr.withUnsafeBufferPointer { Darwin.write(fd, $0.baseAddress, hdr.count) }
        close(fd)
    }
}

let app = NSApplication.shared
let delegate = TrayApp()
app.delegate = delegate
app.run()
