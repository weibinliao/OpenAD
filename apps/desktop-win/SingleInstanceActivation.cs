using System.Runtime.InteropServices;

namespace PermissionProtector.Desktop;

internal static class SingleInstanceActivation
{
    internal const string ActivationMessageName = "OpenAD.Desktop.Activate.7E29C1B4-7D94-4B0D-BCB2-0B3CF582B33A.v1";

    private static readonly IntPtr HwndBroadcast = new(0xffff);
    private const uint AllowAnyProcess = 0xffffffff;
    private const int SwRestore = 9;
    private static readonly int activationMessageId = checked((int)RegisterWindowMessage(ActivationMessageName));

    internal static bool IsActivationMessage(int messageId) =>
        activationMessageId != 0 && messageId == activationMessageId;

    internal static void RequestExistingWindow()
    {
        if (activationMessageId == 0)
        {
            return;
        }

        _ = AllowSetForegroundWindow(AllowAnyProcess);
        for (var attempt = 0; attempt < 5; attempt++)
        {
            _ = PostMessage(HwndBroadcast, (uint)activationMessageId, IntPtr.Zero, IntPtr.Zero);
            if (attempt < 4)
            {
                Thread.Sleep(100);
            }
        }
    }

    internal static void RestoreAndActivateWindow(IntPtr windowHandle)
    {
        if (windowHandle == IntPtr.Zero)
        {
            return;
        }

        var currentThread = GetCurrentThreadId();
        var foregroundWindow = GetForegroundWindow();
        var foregroundThread = foregroundWindow == IntPtr.Zero
            ? 0
            : GetWindowThreadProcessId(foregroundWindow, IntPtr.Zero);
        var attached = foregroundThread != 0 &&
            foregroundThread != currentThread &&
            AttachThreadInput(currentThread, foregroundThread, attach: true);

        try
        {
            if (IsIconic(windowHandle))
            {
                _ = ShowWindow(windowHandle, SwRestore);
            }
            _ = BringWindowToTop(windowHandle);
            _ = SetForegroundWindow(windowHandle);
        }
        finally
        {
            if (attached)
            {
                _ = AttachThreadInput(currentThread, foregroundThread, attach: false);
            }
        }
    }

    [DllImport("user32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
    private static extern uint RegisterWindowMessage(string messageName);

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool PostMessage(IntPtr windowHandle, uint message, IntPtr wParam, IntPtr lParam);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AllowSetForegroundWindow(uint processId);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool IsIconic(IntPtr windowHandle);

    [DllImport("user32.dll")]
    private static extern IntPtr GetForegroundWindow();

    [DllImport("user32.dll")]
    private static extern uint GetWindowThreadProcessId(IntPtr windowHandle, IntPtr processId);

    [DllImport("kernel32.dll")]
    private static extern uint GetCurrentThreadId();

    [DllImport("user32.dll", SetLastError = true)]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool AttachThreadInput(uint idAttach, uint idAttachTo, [MarshalAs(UnmanagedType.Bool)] bool attach);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool ShowWindow(IntPtr windowHandle, int command);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool BringWindowToTop(IntPtr windowHandle);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool SetForegroundWindow(IntPtr windowHandle);
}
