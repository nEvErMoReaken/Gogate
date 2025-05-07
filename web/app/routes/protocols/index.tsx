
import { API } from "../../api";

export const clientLoader = async () => {
    try {
        const response = await API.protocols.getAll();
        if (response.error) {
            return { protocols: [] };
        }
        return response.data;
    } catch (e) {
        console.error(e);
        return { protocols: [] };
    }
};

// ... existing code ...
