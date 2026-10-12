interface WebdavShare {
  name: string;
  path: string;
  readOnly: boolean;
  url: string;
  available: boolean;
  by: string;
}

interface WebdavInfo {
  enabled: boolean;
  user: string;
  baseURL: string;
  shares: WebdavShare[];
}
