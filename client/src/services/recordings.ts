import { apiGet } from "@/lib/api";
import { useAuth } from "@/stores/auth";

export interface Recording {
  id: string;
  session_id: string;
  call_id: string;
  peer: string;
  direction: string;
  duration: number;
  file_path: string;
  file_size: number;
  created_at: string;
}

export const fetchRecordings = (sid: string) =>
  apiGet<{ recordings: Recording[] }>(`/api/sessions/${sid}/recordings`);

export const downloadRecordingUrl = (rid: string) => {
  const token = useAuth.getState().token ?? "";
  return `/api/recordings/${rid}/download?token=${encodeURIComponent(token)}`;
};
