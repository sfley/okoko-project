<script src="<?php echo base_url();?>assets/js/highcharts.js"></script>
<script src="<?php echo base_url();?>assets/js/series-label.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/exporting.js"></script>
<script src="<?php echo base_url();?>assets/js/modules/export-data.js"></script>



<p>
<div class="container text-center">
     
      <footer>
	  <!-- forecast_info":"date|temp|cloud|speed|image|code|unit[|sunrise|sunset|durée du jour]-->
        
		

<span class="badge text-bg-dark">Météokofen : </span> 
<span class="badge text-bg-dark"><?=$info['meteo'][0]?></span>
<span class="badge text-bg-dark"><?=$info['meteo'][1]?>°C</span>
<span class="badge text-bg-dark"><?=$info['meteo'][3]?></span>
<span class="badge text-bg-dark">Lever du soleil : <?=$info['meteo'][7]?></span>
<span class="badge text-bg-dark">Coucher du soleil : <?=$info['meteo'][8]?></span>
<span class="badge text-bg-dark">Durée du jour : <?=$info['meteo'][9]?></span>	
<p>&copy; sfley 2023		
		
		
      </footer>
      </div>
	  
	 
	  
	  
  </body>
  
  
</html>