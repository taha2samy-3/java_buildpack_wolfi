import data from '../data/buildpack.json';
import releases from '../data/releases.json';

export { data, releases };

export const url = (path = '') => {
  const base = import.meta.env.BASE_URL.replace(/\/$/, '');
  return `${base}/${path.replace(/^\//, '')}`;
};

export const builder = `${data.registry}/wolfi-java-builder:${data.latest}`;
