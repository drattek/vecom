package com.vegusa.middleware.integrations.multivende.service.picture;

import com.vegusa.middleware.integrations.multivende.client.picture.MultivendePictureSet;
import com.vegusa.middleware.integrations.multivende.dto.EntriesDto;
import com.vegusa.middleware.integrations.multivende.dto.PictureSet;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

@Service
public class PictureSetService {
    private final MultivendePictureSet multivendePictureSet;

    @Autowired
    public PictureSetService(MultivendePictureSet multivendePictureSet) {
        this.multivendePictureSet = multivendePictureSet;
    }

    public Mono<EntriesDto<PictureSet>> getAllPictureSet(){
        return multivendePictureSet.getAllPictureSet();
    }
}
