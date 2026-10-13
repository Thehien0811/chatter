export type MockMessage = {
  id: string;
  sender: 'me' | 'them';
  text: string;
  time: string;
  status?: 'sent' | 'delivered' | 'seen';
};

export type MockConversation = {
  id: string;
  title: string;
  subtitle: string;
  online: boolean;
  group?: boolean;
  preview: string;
  time: string;
  unread: number;
  messages: MockMessage[];
};

export const initialMockConversations: MockConversation[] = [
  {
    id: 'mai', title: 'Mai Nguyễn', subtitle: 'Đang hoạt động', online: true, preview: 'Vậy hẹn bạn chiều nay nhé ✨', time: '10:42', unread: 2,
    messages: [
      { id: 'mai-1', sender: 'them', text: 'Chào bạn! Mình vừa xem qua bản thiết kế mới rồi.', time: '10:36' },
      { id: 'mai-2', sender: 'me', text: 'Bạn thấy phần giao diện chat thế nào?', time: '10:38', status: 'seen' },
      { id: 'mai-3', sender: 'them', text: 'Mình thích cách bố trí này, nhìn thoáng và dễ dùng hơn nhiều.', time: '10:39' },
      { id: 'mai-4', sender: 'them', text: 'Mình có vài góp ý nhỏ, lát nữa gửi bạn nhé.', time: '10:40' },
      { id: 'mai-5', sender: 'me', text: 'Tuyệt! Cảm ơn bạn. Vậy hẹn bạn chiều nay nhé ✨', time: '10:42', status: 'delivered' },
    ],
  },
  {
    id: 'team', title: 'Nhóm sản phẩm', subtitle: '4 thành viên', online: true, group: true, preview: 'Tuấn: Đã cập nhật lên staging.', time: '09:58', unread: 4,
    messages: [
      { id: 'team-1', sender: 'them', text: 'Mai: Mọi người xem giúp mình flow đăng nhập nhé.', time: '09:45' },
      { id: 'team-2', sender: 'me', text: 'Mình vừa kết nối xong phần API auth.', time: '09:49', status: 'seen' },
      { id: 'team-3', sender: 'them', text: 'Tuấn: Đã cập nhật lên staging.', time: '09:58' },
    ],
  },
  {
    id: 'tuan', title: 'Tuấn Trần', subtitle: 'Hoạt động 15 phút trước', online: false, preview: 'Gửi mình link tài liệu với nha', time: '09:21', unread: 0,
    messages: [
      { id: 'tuan-1', sender: 'them', text: 'Gửi mình link tài liệu với nha', time: '09:21' },
      { id: 'tuan-2', sender: 'me', text: 'Được, mình gửi ngay đây.', time: '09:24', status: 'seen' },
    ],
  },
  {
    id: 'linh', title: 'Linh Phạm', subtitle: 'Đang hoạt động', online: true, preview: 'Ảnh hôm qua đẹp quá 😄', time: 'Hôm qua', unread: 0,
    messages: [
      { id: 'linh-1', sender: 'me', text: 'Cuối tuần mình đi cà phê ở chỗ mới nhé?', time: '20:14', status: 'seen' },
      { id: 'linh-2', sender: 'them', text: 'Ảnh hôm qua đẹp quá 😄', time: '20:19' },
    ],
  },
  {
    id: 'huy', title: 'Huy Lê', subtitle: 'Hoạt động 1 giờ trước', online: false, preview: 'Okie, cảm ơn nhiều!', time: 'Hôm qua', unread: 0,
    messages: [
      { id: 'huy-1', sender: 'them', text: 'Bạn có thể xem qua PR giúp mình không?', time: '17:02' },
      { id: 'huy-2', sender: 'me', text: 'Mình đã review rồi, có để lại vài comment nha.', time: '17:18', status: 'seen' },
      { id: 'huy-3', sender: 'them', text: 'Okie, cảm ơn nhiều!', time: '17:20' },
    ],
  },
];
