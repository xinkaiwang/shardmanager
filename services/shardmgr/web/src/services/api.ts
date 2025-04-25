import axios from 'axios';
import { GetStateResponse } from '../types/api';

const api = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json',
  },
});

export const ping = async (): Promise<string> => {
  const response = await api.get<string>('/ping');
  return response.data;
};

export const getState = async (prefix?: string): Promise<GetStateResponse> => {
  const response = await api.get<GetStateResponse>('/get_state', {
    params: { prefix },
  });
  return response.data;
}; 