namespace PermissionProtector.Desktop;

internal sealed class SingleInstanceLock : IDisposable
{
    // Local is intentional: foreground activation cannot cross Windows sessions,
    // and each interactive user session must retain ownership of its own window.
    internal const string MutexName = @"Local\OpenAD.Desktop.7E29C1B4-7D94-4B0D-BCB2-0B3CF582B33A.SingleInstance.v1";

    private Mutex? mutex;
    private bool ownsMutex;

    private SingleInstanceLock(Mutex mutex)
    {
        this.mutex = mutex;
        ownsMutex = true;
    }

    internal static SingleInstanceLock? TryAcquire(string? mutexName = null)
    {
        var candidate = new Mutex(initiallyOwned: false, mutexName ?? MutexName);
        var acquired = false;
        try
        {
            acquired = candidate.WaitOne(TimeSpan.Zero);
        }
        catch (AbandonedMutexException)
        {
            // Windows releases ownership when a process exits unexpectedly.
            acquired = true;
        }

        if (!acquired)
        {
            candidate.Dispose();
            return null;
        }

        return new SingleInstanceLock(candidate);
    }

    public void Dispose()
    {
        var owned = mutex;
        if (owned is null)
        {
            return;
        }

        mutex = null;
        if (ownsMutex)
        {
            owned.ReleaseMutex();
            ownsMutex = false;
        }
        owned.Dispose();
    }
}
