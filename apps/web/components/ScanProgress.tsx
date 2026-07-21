import { useEffect, useState } from 'react';
import { websocketBase } from '../lib/runtimeApi';

interface ProgressEvent {
  type: string;
  session_id: string;
  items_scanned: number;
  permission_count: number;
  current_path?: string;
  status: string;
  message?: string;
}

interface Props {
  sessionId: string;
  onComplete?: (sessionId: string) => void;
}

export default function ScanProgress({ sessionId, onComplete }: Props) {
  const [progress, setProgress] = useState<ProgressEvent | null>(null);
  const [ws, setWs] = useState<WebSocket | null>(null);

  useEffect(() => {
    if (!sessionId) return;

    const websocket = new WebSocket(`${websocketBase()}/api/scan/ws?scan_id=${encodeURIComponent(sessionId)}`);

    websocket.onmessage = (event) => {
      const data: ProgressEvent = JSON.parse(event.data);
      setProgress(data);

      if (data.status === 'completed' && onComplete) {
        onComplete(data.session_id);
      }
    };

    websocket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    setWs(websocket);

    return () => {
      websocket.close();
    };
  }, [sessionId, onComplete]);

  if (!progress) {
    return <div className="text-center">Connecting...</div>;
  }

  return (
    <div className="bg-white p-6 rounded-lg shadow">
      <h3 className="text-lg font-semibold mb-4">Scan Progress</h3>

      <div className="space-y-4">
        <div className="flex justify-between">
          <span>Status:</span>
          <span className={`font-medium ${
            progress.status === 'completed' ? 'text-green-600' :
            progress.status === 'failed' ? 'text-red-600' :
            'text-blue-600'
          }`}>
            {progress.status}
          </span>
        </div>

        {progress.items_scanned > 0 && (
          <div className="flex justify-between">
            <span>Items Scanned:</span>
            <span>{progress.items_scanned}</span>
          </div>
        )}

        {progress.permission_count > 0 && (
          <div className="flex justify-between">
            <span>Permissions Found:</span>
            <span>{progress.permission_count}</span>
          </div>
        )}

        {progress.current_path && (
          <div>
            <span className="text-sm text-gray-600">Current Path:</span>
            <div className="text-sm font-mono bg-gray-100 p-2 rounded mt-1 truncate">
              {progress.current_path}
            </div>
          </div>
        )}

        {progress.message && (
          <div className="text-sm text-gray-700 bg-gray-50 p-2 rounded">
            {progress.message}
          </div>
        )}
      </div>
    </div>
  );
}
