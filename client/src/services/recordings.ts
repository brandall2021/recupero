import { apiGet } from "@/lib/api";

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

export const downloadRecordingUrl = (rid: string) =>
  `/api/recordings/${rid}/download`;
