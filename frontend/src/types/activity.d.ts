interface ActivityEntry {
  id: number;
  time: string;
  userID: number;
  username: string;
  action: string;
  path: string;
  dest?: string;
  detail?: string;
  ip: string;
}

interface ActivityPage {
  items: ActivityEntry[];
  total: number;
}
